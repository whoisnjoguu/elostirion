package reader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v90/github"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// githubReader reads repository files through the GitHub API
type githubReader struct {
	client *github.Client
	owner  string
	name   string
	ref    string // empty means the repository default branch

	once     sync.Once
	blobs    map[string]string // path -> blob SHA
	byDir    map[string][]DirEntry
	fallback bool // tree truncated or unavailable: use the Contents API
}

// newGitHubReader builds a reader for a single repository.
func newGitHubReader(repo model.Repo, cfg forge.Config) (*githubReader, error) {
	client, err := githubClient(cfg)
	if err != nil {
		return nil, err
	}
	return &githubReader{client: client, owner: repo.Owner, name: repo.Name, ref: repo.Ref}, nil
}

// githubClient builds a GitHub client from the config
func githubClient(cfg forge.Config) (*github.Client, error) {
	var opts []github.ClientOptionsFunc
	if cfg.Token != "" {
		opts = append(opts, github.WithAuthToken(cfg.Token))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("forge/github: client: %w", err)
	}
	return client, nil
}

func (r *githubReader) contentOpts() *github.RepositoryContentGetOptions {
	if r.ref == "" {
		return nil
	}
	return &github.RepositoryContentGetOptions{Ref: r.ref}
}

// ensureTree fetches the recursive repository tree once and builds the path index
func (r *githubReader) ensureTree(ctx context.Context) {
	r.once.Do(func() {
		ref := r.ref
		if ref == "" {
			repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
			if err != nil {
				r.fallback = true
				return
			}
			ref = repo.GetDefaultBranch()
		}
		commit, _, err := r.client.Repositories.GetCommit(ctx, r.owner, r.name, ref, nil)
		if err != nil {
			r.fallback = true
			return
		}
		tree, _, err := r.client.Git.GetTree(ctx, r.owner, r.name, commit.GetCommit().GetTree().GetSHA(), true)
		if err != nil {
			r.fallback = true
			return
		}
		if tree.GetTruncated() {
			r.fallback = true // too large for one tree call; fall back per path
			return
		}
		r.blobs = make(map[string]string)
		r.byDir = make(map[string][]DirEntry)
		seen := map[string]bool{} // dedupe directory entries across sibling paths
		for _, e := range tree.Entries {
			if e.GetType() != "blob" {
				continue // directories are derived from blob paths below
			}
			path := e.GetPath()
			r.blobs[path] = e.GetSHA()
			// Register the blob and every ancestor directory under its parent.
			segs := strings.Split(path, "/")
			for i := range segs {
				parent := strings.Join(segs[:i], "/")
				name := segs[i]
				if key := parent + "\x00" + name; seen[key] {
					continue
				} else {
					seen[key] = true
				}
				r.byDir[parent] = append(r.byDir[parent], DirEntry{Name: name, IsDir: i < len(segs)-1})
			}
		}
	})
}

// GetFile fetches a file's contents
func (r *githubReader) GetFile(ctx context.Context, path string) ([]byte, error) {
	r.ensureTree(ctx)
	if r.fallback {
		return r.getFileContents(ctx, path)
	}
	sha, ok := r.blobs[path]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	data, _, err := r.client.Git.GetBlobRaw(ctx, r.owner, r.name, sha)
	if err != nil {
		return nil, classifyGitHubErr(err, path)
	}
	return data, nil
}

// ListFiles lists the immediate entries under dir
func (r *githubReader) ListFiles(ctx context.Context, dir string) ([]DirEntry, error) {
	r.ensureTree(ctx)
	if r.fallback {
		return r.listFilesContents(ctx, dir)
	}
	return append([]DirEntry(nil), r.byDir[dir]...), nil
}

// getFileContents fetches and decodes a single file via the Contents API.
func (r *githubReader) getFileContents(ctx context.Context, path string) ([]byte, error) {
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, r.contentOpts())
	if err != nil {
		return nil, classifyGitHubErr(err, path)
	}
	if file == nil { // path resolved to a directory
		return nil, &fs.PathError{Op: "getfile", Path: path, Err: fs.ErrNotExist}
	}
	content, err := file.GetContent()
	if err != nil {
		return nil, fmt.Errorf("forge/github: decode %s: %w", path, err)
	}
	return []byte(content), nil
}

// listFilesContents lists dir via the Contents API.
func (r *githubReader) listFilesContents(ctx context.Context, dir string) ([]DirEntry, error) {
	_, contents, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, dir, r.contentOpts())
	if err != nil {
		return nil, classifyGitHubErr(err, dir)
	}
	entries := make([]DirEntry, 0, len(contents))
	for _, c := range contents {
		entries = append(entries, DirEntry{Name: c.GetName(), IsDir: c.GetType() == "dir"})
	}
	return entries, nil
}

// ListOrgRepos enumerates every repository in an organisation.
func ListOrgRepos(ctx context.Context, provider, org string, cfg forge.Config) ([]model.Repo, error) {
	if provider != "github" {
		return nil, fmt.Errorf("forge: --org enumeration is only supported for github, not %q", provider)
	}
	client, err := githubClient(cfg)
	if err != nil {
		return nil, err
	}
	return githubOrgRepos(ctx, client, org)
}

// githubOrgRepos pages through an org's repositories via the GitHub API.
func githubOrgRepos(ctx context.Context, client *github.Client, org string) ([]model.Repo, error) {
	opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var repos []model.Repo
	for {
		page, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return nil, classifyGitHubErr(err, org)
		}
		for _, r := range page {
			repos = append(repos, model.Repo{
				Provider: "github",
				Owner:    org,
				Name:     r.GetName(),
				Ref:      r.GetDefaultBranch(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}

// classifyGitHubErr turns an API error into an AuthError
func classifyGitHubErr(err error, path string) error {
	var er *github.ErrorResponse
	if errors.As(err, &er) && er.Response != nil {
		switch er.Response.StatusCode {
		case http.StatusUnauthorized:
			return &AuthError{Provider: "github", Msg: "bad credentials; check GITHUB_TOKEN"}
		case http.StatusForbidden:
			return &AuthError{Provider: "github", Msg: "forbidden; token may lack the required scopes (repo, read:org)"}
		case http.StatusNotFound:
			return &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
		}
	}
	return fmt.Errorf("forge/github: %w", err)
}
