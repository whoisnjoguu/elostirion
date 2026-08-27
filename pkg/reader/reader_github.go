package reader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/google/go-github/v90/github"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// githubReader reads repository files through the GitHub Contents API.
type githubReader struct {
	client *github.Client
	owner  string
	name   string
	ref    string // empty means the repository default branch
}

// newGitHubReader builds a Contents-API reader for a single repository.
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

// GetFile fetches and decodes a single file's contents.
func (r *githubReader) GetFile(ctx context.Context, path string) ([]byte, error) {
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

// ListFiles lists the immediate entries under dir ("" for the repository root).
func (r *githubReader) ListFiles(ctx context.Context, dir string) ([]DirEntry, error) {
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
