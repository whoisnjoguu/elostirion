package cli

import (
	"bytes"
	"io"
	"io/fs"
	"time"
)

// overlayFS serves in-memory edited content over a base filesystem so recipes
// running later in a plan see the edits of earlier ones
type overlayFS struct {
	base  fs.FS
	edits map[string][]byte
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if content, ok := o.edits[name]; ok {
		return &memFile{name: name, r: bytes.NewReader(content), size: int64(len(content))}, nil
	}
	return o.base.Open(name)
}

// memFile is a read-only fs.File over a byte slice
type memFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (m *memFile) Stat() (fs.FileInfo, error) { return memInfo{name: m.name, size: m.size}, nil }
func (m *memFile) Read(p []byte) (int, error) { return m.r.Read(p) }
func (m *memFile) Close() error               { return nil }

type memInfo struct {
	name string
	size int64
}

func (i memInfo) Name() string       { return i.name }
func (i memInfo) Size() int64        { return i.size }
func (i memInfo) Mode() fs.FileMode  { return 0o644 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }

var _ io.Reader = (*memFile)(nil)
