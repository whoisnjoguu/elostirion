package reader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func newTestBitbucket(server *httptest.Server, ref string) *bitbucketReader {
	return &bitbucketReader{
		http:      server.Client(),
		baseURL:   server.URL,
		token:     "t",
		workspace: "acme",
		repo:      "api",
		ref:       ref,
	}
}

func TestBitbucketGetFile(t *testing.T) {
	want := "FROM golang:1.25\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			t.Errorf("missing bearer auth: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/2.0/repositories/acme/api/src/main/Dockerfile" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		io.WriteString(w, want)
	}))
	defer server.Close()

	got, err := newTestBitbucket(server, "main").GetFile(context.Background(), "Dockerfile")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got) != want {
		t.Errorf("GetFile = %q, want %q", got, want)
	}
}

func TestBitbucketResolveRefAndGetFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2.0/repositories/acme/api":
			io.WriteString(w, `{"mainbranch":{"name":"develop"}}`)
		case "/2.0/repositories/acme/api/src/develop/go.mod":
			io.WriteString(w, "module x\n")
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	got, err := newTestBitbucket(server, "").GetFile(context.Background(), "go.mod")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got) != "module x\n" {
		t.Errorf("GetFile = %q", got)
	}
}

func TestBitbucketListFilesPaginated(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			io.WriteString(w, `{"values":[{"path":"cmd","type":"commit_directory"}]}`)
			return
		}
		next := fmt.Sprintf("%s/2.0/repositories/acme/api/src/main/?page=2", server.URL)
		fmt.Fprintf(w, `{"values":[{"path":"go.mod","type":"commit_file"}],"next":%q}`, next)
	}))
	defer server.Close()

	entries, err := newTestBitbucket(server, "main").ListFiles(context.Background(), "")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	if entries[0].Name != "go.mod" || entries[0].IsDir {
		t.Errorf("entry[0] = %+v", entries[0])
	}
	if entries[1].Name != "cmd" || !entries[1].IsDir {
		t.Errorf("entry[1] = %+v", entries[1])
	}
}

func TestBitbucketNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := newTestBitbucket(server, "main").GetFile(context.Background(), "missing")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("GetFile err = %v, want fs.ErrNotExist", err)
	}
}

func TestBitbucketAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := newTestBitbucket(server, "main").ListFiles(context.Background(), "")
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("ListFiles err = %v, want *AuthError", err)
	}
}

func TestBitbucketNoTokenAllowsPublicRepos(t *testing.T) {
	r, err := For(model.Repo{Provider: "bitbucket", Owner: "acme", Name: "api"}, forge.Config{})
	if err != nil {
		t.Fatalf("For without token = %v, want nil (public repos need no token)", err)
	}
	if r == nil {
		t.Fatal("For returned nil reader")
	}
}
