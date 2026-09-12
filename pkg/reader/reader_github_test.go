package reader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v90/github"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// testGitHubClient points a go-github client at a test server so the transport
// is fully mocked and no live calls are made.
func testGitHubClient(t *testing.T, server *httptest.Server) *github.Client {
	t.Helper()
	base := server.URL + "/"
	c, err := github.NewClient(github.WithHTTPClient(server.Client()), github.WithURLs(&base, &base))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

// treeServer mocks the Git Trees flow
func treeServer(t *testing.T, files map[string]string, blobHits *int) *httptest.Server {
	t.Helper()
	// deterministic blob SHA per path
	shaOf := func(path string) string { return "sha-" + path }
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/api":
			io.WriteString(w, `{"default_branch":"main"}`)
		case r.URL.Path == "/repos/acme/api/commits/main":
			io.WriteString(w, `{"sha":"c1","commit":{"tree":{"sha":"t1"}}}`)
		case r.URL.Path == "/repos/acme/api/git/trees/t1":
			var b strings.Builder
			b.WriteString(`{"sha":"t1","truncated":false,"tree":[`)
			first := true
			for path := range files {
				if !first {
					b.WriteString(",")
				}
				first = false
				fmt.Fprintf(&b, `{"path":%q,"type":"blob","sha":%q}`, path, shaOf(path))
			}
			b.WriteString("]}")
			io.WriteString(w, b.String())
		case strings.HasPrefix(r.URL.Path, "/repos/acme/api/git/blobs/"):
			if blobHits != nil {
				*blobHits++
			}
			sha := strings.TrimPrefix(r.URL.Path, "/repos/acme/api/git/blobs/")
			for path, content := range files {
				if shaOf(path) == sha {
					io.WriteString(w, content)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
}

func TestGitHubReaderGetFile(t *testing.T) {
	want := "module example.com/svc\n\ngo 1.25\n"
	var blobHits int
	server := treeServer(t, map[string]string{"go.mod": want}, &blobHits)
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api", ref: "main"}
	got, err := reader.GetFile(context.Background(), "go.mod")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got) != want {
		t.Errorf("GetFile = %q, want %q", got, want)
	}
	if blobHits != 1 {
		t.Errorf("blob fetches = %d, want 1", blobHits)
	}
}

// TestGitHubReaderMissIsFree proves a missing file is answered from the cached tree with no blob fetch
func TestGitHubReaderMissIsFree(t *testing.T) {
	var blobHits int
	server := treeServer(t, map[string]string{"go.mod": "module x\n"}, &blobHits)
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api", ref: "main"}
	for _, miss := range []string{"pyproject.toml", "Dockerfile", "docker/Dockerfile", ".gitlab-ci.yml"} {
		if _, err := reader.GetFile(context.Background(), miss); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("GetFile(%q) err = %v, want fs.ErrNotExist", miss, err)
		}
	}
	if blobHits != 0 {
		t.Errorf("blob fetches for missing files = %d, want 0", blobHits)
	}
}

func TestGitHubReaderListFiles(t *testing.T) {
	server := treeServer(t, map[string]string{
		"go.mod":       "module x\n",
		"cmd/elo/main": "package main\n",
	}, nil)
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api", ref: "main"}
	entries, err := reader.ListFiles(context.Background(), "")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := map[string]bool{"go.mod": false, "cmd": true}
	if len(entries) != len(want) {
		t.Fatalf("ListFiles = %v", entries)
	}
	for _, e := range entries {
		isDir, ok := want[e.Name]
		if !ok || isDir != e.IsDir {
			t.Errorf("unexpected entry %+v", e)
		}
	}
}

func TestGitHubReaderNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api"}
	_, err := reader.GetFile(context.Background(), "go.mod")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("GetFile err = %v, want fs.ErrNotExist", err)
	}
}

func TestGitHubReaderAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"Bad credentials"}`)
	}))
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api"}
	_, err := reader.ListFiles(context.Background(), "")
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("ListFiles err = %v, want *AuthError", err)
	}
}

func TestGitHubOrgRepos(t *testing.T) {
	var pages int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/acme/repos" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		pages++
		if r.URL.Query().Get("page") == "2" {
			io.WriteString(w, `[{"name":"worker","default_branch":"main"}]`)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/orgs/acme/repos?page=2>; rel="next"`, server.URL))
		io.WriteString(w, `[{"name":"api","default_branch":"trunk"}]`)
	}))
	defer server.Close()

	repos, err := githubOrgRepos(context.Background(), testGitHubClient(t, server), "acme")
	if err != nil {
		t.Fatalf("githubOrgRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("repos = %v", repos)
	}
	if repos[0].Name != "api" || repos[0].Ref != "trunk" || repos[0].Owner != "acme" {
		t.Errorf("repo[0] = %+v", repos[0])
	}
	if repos[1].Name != "worker" {
		t.Errorf("repo[1] = %+v", repos[1])
	}
	if pages != 2 {
		t.Errorf("expected 2 pages, got %d", pages)
	}
}

func TestGitHubNoTokenAllowsPublicRepos(t *testing.T) {
	r, err := For(model.Repo{Provider: "github", Owner: "acme", Name: "api"}, forge.Config{})
	if err != nil {
		t.Fatalf("For without token = %v, want nil (public repos need no token)", err)
	}
	if r == nil {
		t.Fatal("For returned nil reader")
	}
}

func TestForUnknownProvider(t *testing.T) {
	_, err := For(model.Repo{Provider: "svn"}, forge.Config{Token: "t"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
