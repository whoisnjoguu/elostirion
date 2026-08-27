package reader

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
)

// fakeReader serves a fixed set of files for testing the fs.FS adapter.
type fakeReader struct {
	files map[string][]byte
	dirs  map[string][]DirEntry
}

func (f fakeReader) GetFile(_ context.Context, path string) ([]byte, error) {
	if data, ok := f.files[path]; ok {
		return data, nil
	}
	return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
}

func (f fakeReader) ListFiles(_ context.Context, dir string) ([]DirEntry, error) {
	return f.dirs[dir], nil
}

func TestFSOpenFile(t *testing.T) {
	r := fakeReader{files: map[string][]byte{"go.mod": []byte("module x\n")}}
	fsys := FS(context.Background(), r)

	f, err := fsys.Open("go.mod")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "module x\n" {
		t.Errorf("content = %q", data)
	}
}

func TestFSOpenMissing(t *testing.T) {
	fsys := FS(context.Background(), fakeReader{files: map[string][]byte{}})
	_, err := fsys.Open("nope")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open err = %v, want fs.ErrNotExist", err)
	}
}

func TestFSReadRoot(t *testing.T) {
	r := fakeReader{dirs: map[string][]DirEntry{
		"": {{Name: "go.mod"}, {Name: "cmd", IsDir: true}},
	}}
	entries, err := fs.ReadDir(FS(context.Background(), r), ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
}
