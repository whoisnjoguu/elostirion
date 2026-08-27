package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// defaultBitbucketAPI is the public Bitbucket Cloud API base.
const defaultBitbucketAPI = "https://api.bitbucket.org"

// bitbucketReader reads repository files through the Bitbucket Source API.
type bitbucketReader struct {
	http      *http.Client
	baseURL   string
	token     string
	workspace string
	repo      string
	ref       string // resolved lazily from the repo's main branch when empty
}

// newBitbucketReader builds a Source-API reader for a single repository
func newBitbucketReader(repo model.Repo, cfg forge.Config) (*bitbucketReader, error) {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBitbucketAPI
	}
	return &bitbucketReader{
		http:      http.DefaultClient,
		baseURL:   strings.TrimRight(base, "/"),
		token:     cfg.Token,
		workspace: repo.Owner,
		repo:      repo.Name,
		ref:       repo.Ref,
	}, nil
}

// resolveRef returns the configured ref or the repository's main branch.
func (b *bitbucketReader) resolveRef(ctx context.Context) (string, error) {
	if b.ref != "" {
		return b.ref, nil
	}
	var meta struct {
		Mainbranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
	}
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s", b.baseURL, b.workspace, b.repo)
	if err := b.getJSON(ctx, url, &meta); err != nil {
		return "", err
	}
	if meta.Mainbranch.Name == "" {
		return "", fmt.Errorf("forge/bitbucket: no main branch for %s/%s", b.workspace, b.repo)
	}
	b.ref = meta.Mainbranch.Name
	return b.ref, nil
}

// GetFile fetches the raw contents of a file.
func (b *bitbucketReader) GetFile(ctx context.Context, filePath string) ([]byte, error) {
	ref, err := b.resolveRef(ctx)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/src/%s/%s",
		b.baseURL, b.workspace, b.repo, ref, strings.TrimLeft(filePath, "/"))
	resp, err := b.do(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := b.checkStatus(resp, filePath); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// ListFiles lists the immediate entries under dir ("" for the repository root).
func (b *bitbucketReader) ListFiles(ctx context.Context, dir string) ([]DirEntry, error) {
	ref, err := b.resolveRef(ctx)
	if err != nil {
		return nil, err
	}
	p := strings.Trim(dir, "/")
	url := fmt.Sprintf("%s/2.0/repositories/%s/%s/src/%s/%s/",
		b.baseURL, b.workspace, b.repo, ref, p)

	var entries []DirEntry
	for url != "" {
		var page struct {
			Values []struct {
				Path string `json:"path"`
				Type string `json:"type"`
			} `json:"values"`
			Next string `json:"next"`
		}
		if err := b.getJSON(ctx, url, &page); err != nil {
			return nil, err
		}
		for _, v := range page.Values {
			entries = append(entries, DirEntry{
				Name:  path.Base(v.Path),
				IsDir: v.Type == "commit_directory",
			})
		}
		url = page.Next
	}
	return entries, nil
}

// getJSON performs an authenticated GET and decodes the JSON body.
func (b *bitbucketReader) getJSON(ctx context.Context, url string, out any) error {
	resp, err := b.do(ctx, url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := b.checkStatus(resp, url); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// do issues a GET request, authenticating only when a token is configured.
func (b *bitbucketReader) do(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("forge/bitbucket: request: %w", err)
	}
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forge/bitbucket: get: %w", err)
	}
	return resp, nil
}

// checkStatus maps HTTP status codes to auth, not-exist, or generic errors.
func (b *bitbucketReader) checkStatus(resp *http.Response, target string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return &AuthError{Provider: "bitbucket", Msg: "bad credentials; check BITBUCKET_TOKEN"}
	case http.StatusForbidden:
		return &AuthError{Provider: "bitbucket", Msg: "forbidden; token may lack the required scopes (repository:read)"}
	case http.StatusNotFound:
		return &fs.PathError{Op: "open", Path: target, Err: fs.ErrNotExist}
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("forge/bitbucket: %s: unexpected status %d: %s",
			target, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
