// Package vcs provides filesystem access to local and remote repositories.
package vcs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/google/go-github/v90/github"
)

// GitHubFS is a read-only fs.FS over a repository tree fetched through the
// GitHub Git Data API, so scanners can read remote repos without cloning
type GitHubFS struct {
	ctx       context.Context
	client    *github.Client
	owner     string
	name      string
	rootTree  string
	treeCache map[string]*github.Tree
}

// NewGitHubFS resolves ref and returns a filesystem rooted at that commit's tree
func NewGitHubFS(ctx context.Context, client *github.Client, owner, name, ref string) (*GitHubFS, error) {
	if ref == "" {
		repo, _, err := client.Repositories.Get(ctx, owner, name)
		if err != nil {
			return nil, fmt.Errorf("vcs/github: get repo %s/%s: %w", owner, name, err)
		}
		ref = repo.GetDefaultBranch()
	}
	commit, _, err := client.Repositories.GetCommit(ctx, owner, name, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("vcs/github: resolve ref %q: %w", ref, err)
	}
	treeSHA := commit.GetCommit().GetTree().GetSHA()
	if treeSHA == "" {
		return nil, fmt.Errorf("vcs/github: ref %q has no tree", ref)
	}
	return &GitHubFS{
		ctx:       ctx,
		client:    client,
		owner:     owner,
		name:      name,
		rootTree:  treeSHA,
		treeCache: map[string]*github.Tree{},
	}, nil
}

// Open fetches the blob at name
func (g *GitHubFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	entry, err := g.lookup(name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	data, _, err := g.client.Git.GetBlobRaw(g.ctx, g.owner, g.name, entry.GetSHA())
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &ghFile{name: name, r: bytes.NewReader(data), size: int64(len(data))}, nil
}

// lookup walks the path's directories through cached subtrees and returns the blob entry
func (g *GitHubFS) lookup(name string) (*github.TreeEntry, error) {
	parts := strings.Split(name, "/")
	dirs, base := parts[:len(parts)-1], parts[len(parts)-1]

	sha := g.rootTree
	for _, d := range dirs {
		tree, err := g.tree(sha)
		if err != nil {
			return nil, err
		}
		entry := findEntry(tree, d, "tree")
		if entry == nil {
			return nil, nil
		}
		sha = entry.GetSHA()
	}
	tree, err := g.tree(sha)
	if err != nil {
		return nil, err
	}
	return findEntry(tree, base, "blob"), nil
}

func (g *GitHubFS) tree(sha string) (*github.Tree, error) {
	if t, ok := g.treeCache[sha]; ok {
		return t, nil
	}
	t, _, err := g.client.Git.GetTree(g.ctx, g.owner, g.name, sha, false)
	if err != nil {
		return nil, fmt.Errorf("vcs/github: get tree %s: %w", sha, err)
	}
	g.treeCache[sha] = t
	return t, nil
}

func findEntry(t *github.Tree, name, entryType string) *github.TreeEntry {
	for _, e := range t.Entries {
		if e.GetPath() == name && e.GetType() == entryType {
			return e
		}
	}
	return nil
}

// ghFile is a read-only fs.File over fetched blob content.
type ghFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *ghFile) Stat() (fs.FileInfo, error) { return ghInfo{name: f.name, size: f.size}, nil }
func (f *ghFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *ghFile) Close() error               { return nil }

type ghInfo struct {
	name string
	size int64
}

func (i ghInfo) Name() string       { return i.name }
func (i ghInfo) Size() int64        { return i.size }
func (i ghInfo) Mode() fs.FileMode  { return 0o644 }
func (i ghInfo) ModTime() time.Time { return time.Time{} }
func (i ghInfo) IsDir() bool        { return false }
func (i ghInfo) Sys() any           { return nil }
