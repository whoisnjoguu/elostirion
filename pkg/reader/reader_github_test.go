package reader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

func TestGitHubReaderGetFile(t *testing.T) {
	want := "module example.com/svc\n\ngo 1.25\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/api/contents/go.mod" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		enc := base64.StdEncoding.EncodeToString([]byte(want))
		fmt.Fprintf(w, `{"type":"file","name":"go.mod","path":"go.mod","encoding":"base64","content":%q}`, enc)
	}))
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api"}
	got, err := reader.GetFile(context.Background(), "go.mod")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got) != want {
		t.Errorf("GetFile = %q, want %q", got, want)
	}
}

func TestGitHubReaderListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[
			{"type":"file","name":"go.mod","path":"go.mod"},
			{"type":"file","name":"Dockerfile","path":"Dockerfile"},
			{"type":"dir","name":"cmd","path":"cmd"}
		]`)
	}))
	defer server.Close()

	reader := &githubReader{client: testGitHubClient(t, server), owner: "acme", name: "api"}
	entries, err := reader.ListFiles(context.Background(), "")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := map[string]bool{"go.mod": false, "Dockerfile": false, "cmd": true}
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
	r, err := ReaderFor(model.Repo{Provider: "github", Owner: "acme", Name: "api"}, forge.Config{})
	if err != nil {
		t.Fatalf("ReaderFor without token = %v, want nil (public repos need no token)", err)
	}
	if r == nil {
		t.Fatal("ReaderFor returned nil reader")
	}
}

func TestReaderForUnknownProvider(t *testing.T) {
	_, err := ReaderFor(model.Repo{Provider: "svn"}, forge.Config{Token: "t"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
