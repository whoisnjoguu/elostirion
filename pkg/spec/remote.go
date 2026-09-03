package spec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/go-github/v90/github"

	"github.com/whoisnjoguu/elostirion/pkg/vcs"
)

// gitSource is a parsed reference to a spec file inside a GitHub repo
type gitSource struct {
	provider string
	owner    string
	repo     string
	path     string
	ref      string
}

// loadGit resolves a git::<url>[//path][?ref=]|[@ref] source into a parsed spec.
func loadGit(source string) (*Spec, error) {
	gs, err := parseGitSource(source)
	if err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	return fetchGitHubSpec(gs, source)
}

// loadURL resolves an http(s):// spec source
func loadURL(source string) (*Spec, error) {
	if gs, ok := parseGitHubWebURL(source); ok {
		return fetchGitHubSpec(gs, source)
	}
	return fetchHTTPSpec(source)
}

// fetchGitHubSpec reads a spec file through the GitHub API
func fetchGitHubSpec(gs gitSource, source string) (*Spec, error) {
	if gs.provider != "github" {
		return nil, fmt.Errorf("spec: remote git host for %q is not supported yet; only github.com is (got provider %q)", source, gs.provider)
	}
	if gs.path == "" {
		return nil, fmt.Errorf("spec: %q has no spec file path; point at the file, e.g. //path/to/fleet.yaml", source)
	}

	ctx := context.Background()
	var opts []github.ClientOptionsFunc
	if token := githubToken(); token != "" {
		opts = append(opts, github.WithAuthToken(token))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("spec: github client for %q: %w", source, err)
	}

	fsys, err := vcs.NewGitHubFS(ctx, client, gs.owner, gs.repo, gs.ref)
	if err != nil {
		return nil, fmt.Errorf("spec: resolve %s (ref %s): %w", source, refLabel(gs.ref), err)
	}

	f, err := fsys.Open(gs.path)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s from %s (ref %s): %w", gs.path, source, refLabel(gs.ref), err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s from %s (ref %s): %w", gs.path, source, refLabel(gs.ref), err)
	}
	return Parse(data)
}

// fetchHTTPSpec downloads a spec from an arbitrary http(s) URL. It sends a
func fetchHTTPSpec(source string) (*Spec, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("spec: request %q: %w", source, err)
	}
	req.Header.Set("Accept", "application/yaml, text/yaml, text/plain, */*")
	if token := os.Getenv("ELO_SPEC_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spec: fetch %q: %w", source, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spec: fetch %q: unexpected status %s", source, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("spec: read body from %q: %w", source, err)
	}
	return Parse(data)
}

// parseGitSource splits a git:: source into repository, spec path, and ref.
func parseGitSource(source string) (gitSource, error) {
	raw := strings.TrimPrefix(source, "git::")

	raw, ref, err := splitRef(raw)
	if err != nil {
		return gitSource{}, fmt.Errorf("parse %q: %w", source, err)
	}

	urlPart, filePath := splitSubpath(raw)

	provider, owner, repo, err := vcs.ParseRemote(urlPart)
	if err != nil {
		return gitSource{}, fmt.Errorf("parse %q: %w", source, err)
	}

	filePath = cleanSpecPath(filePath)
	if filePath == "" {
		return gitSource{}, fmt.Errorf("parse %q: missing spec file path; point at the file with //path/to/fleet.yaml", source)
	}

	return gitSource{provider: provider, owner: owner, repo: repo, path: filePath, ref: ref}, nil
}

// parseGitHubWebURL parses the GitHub URL
func parseGitHubWebURL(source string) (gitSource, bool) {
	u, err := url.Parse(source)
	if err != nil || u.Host != "github.com" {
		return gitSource{}, false
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	// OWNER/REPO/(blob|raw)/REF/PATH...
	if len(segs) < 5 || (segs[2] != "blob" && segs[2] != "raw") {
		return gitSource{}, false
	}
	return gitSource{
		provider: "github",
		owner:    segs[0],
		repo:     strings.TrimSuffix(segs[1], ".git"),
		ref:      segs[3],
		path:     cleanSpecPath(strings.Join(segs[4:], "/")),
	}, true
}

// splitRef extracts a ref from a "?ref=" query or a trailing "@REF"
func splitRef(raw string) (rest, ref string, err error) {
	if i := strings.Index(raw, "?"); i >= 0 {
		q, perr := url.ParseQuery(raw[i+1:])
		if perr != nil {
			return "", "", fmt.Errorf("invalid query %q: %w", raw[i+1:], perr)
		}
		ref = q.Get("ref")
		raw = raw[:i]
	}
	if ref == "" {
		if i := strings.LastIndex(raw, "@"); i >= 0 {
			if cand := raw[i+1:]; cand != "" && !strings.ContainsAny(cand, "/:") {
				ref = cand
				raw = raw[:i]
			}
		}
	}
	return raw, ref, nil
}

// splitSubpath separates the repository URL from the in-repo file path
func splitSubpath(raw string) (urlPart, filePath string) {
	from := 0
	if i := strings.Index(raw, "://"); i >= 0 {
		from = i + 3
	}
	if i := strings.Index(raw[from:], "//"); i >= 0 {
		idx := from + i
		return raw[:idx], raw[idx+2:]
	}
	return raw, ""
}

// cleanSpecPath normalizes an in-repo file path to a valid slash path.
func cleanSpecPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	return p
}

// returns the GITHUB_TOKEN set
func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// refLabel renders a ref for error messages, naming the default branch when empty.
func refLabel(ref string) string {
	if ref == "" {
		return "default branch"
	}
	return ref
}
