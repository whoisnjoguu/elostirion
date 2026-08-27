package reader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// RemoteReader reads repository files from a provider API without a local clone
type RemoteReader interface {
	GetFile(ctx context.Context, path string) ([]byte, error)      // returns the raw contents of the file at path on the repo's ref
	ListFiles(ctx context.Context, dir string) ([]DirEntry, error) // eturns the immediate entries under dir
}

// DirEntry is a single entry returned by RemoteReader.ListFiles.
type DirEntry struct {
	Name  string
	IsDir bool
}

// AuthError marks a failure to authenticate against a provider
type AuthError struct {
	Provider string
	Msg      string
}

func (e *AuthError) Error() string { return fmt.Sprintf("forge/%s: %s", e.Provider, e.Msg) }

// ReaderFor returns a RemoteReader for the repo's provider.
func ReaderFor(repo model.Repo, cfg forge.Config) (RemoteReader, error) {
	switch repo.Provider {
	case "github":
		return newGitHubReader(repo, cfg)
	case "bitbucket":
		return newBitbucketReader(repo, cfg)
	default:
		return nil, forge.ErrUnknownProvider(repo.Provider)
	}
}

// FS adapts a RemoteReader to an fs.FS so the scan layer can drive existing scanners against remote content
func FS(ctx context.Context, r RemoteReader) fs.FS {
	return &remoteFS{ctx: ctx, reader: r}
}

type remoteFS struct {
	ctx    context.Context
	reader RemoteReader
}

// Open fetches name through the reader, mapping a missing file to fs.ErrNotExist.
func (r *remoteFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		entries, err := r.reader.ListFiles(r.ctx, "")
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return newRemoteDir(name, entries), nil
	}
	data, err := r.reader.GetFile(r.ctx, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &remoteFile{name: name, r: bytes.NewReader(data), size: int64(len(data))}, nil
}

// remoteFile is a read-only fs.File over fetched file content.
type remoteFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *remoteFile) Stat() (fs.FileInfo, error) { return remoteInfo{name: f.name, size: f.size}, nil }
func (f *remoteFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *remoteFile) Close() error               { return nil }

// remoteDir is a read-only directory file backed by a listing.
type remoteDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func newRemoteDir(name string, entries []DirEntry) *remoteDir {
	out := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, remoteInfo{name: e.Name, dir: e.IsDir})
	}
	return &remoteDir{name: name, entries: out}
}

func (d *remoteDir) Stat() (fs.FileInfo, error) { return remoteInfo{name: d.name, dir: true}, nil }
func (d *remoteDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}
func (d *remoteDir) Close() error { return nil }

func (d *remoteDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		rest := d.entries[d.offset:]
		d.offset = len(d.entries)
		return rest, nil
	}
	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := min(d.offset+n, len(d.entries))
	out := d.entries[d.offset:end]
	d.offset = end
	return out, nil
}

// remoteInfo satisfies fs.FileInfo and fs.DirEntry for remote entries.
type remoteInfo struct {
	name string
	size int64
	dir  bool
}

func (i remoteInfo) Name() string { return i.name }
func (i remoteInfo) Size() int64  { return i.size }
func (i remoteInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (i remoteInfo) ModTime() time.Time         { return time.Time{} }
func (i remoteInfo) IsDir() bool                { return i.dir }
func (i remoteInfo) Sys() any                   { return nil }
func (i remoteInfo) Type() fs.FileMode          { return i.Mode().Type() }
func (i remoteInfo) Info() (fs.FileInfo, error) { return i, nil }
