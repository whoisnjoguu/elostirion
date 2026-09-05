package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// buildTree creates each "relDir/marker" file under root.
func buildTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, marker := range files {
		dir := filepath.Join(root, rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, marker), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// relSet returns the discovered dirs as paths relative to root, sorted.
func relSet(t *testing.T, root string, dirs []string) []string {
	t.Helper()
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		rel, err := filepath.Rel(root, d)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCollectReposRecursive(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, map[string]string{
		"svc-go":         "go.mod",         // depth 1
		"svc-go/backend": "pyproject.toml", // nested project of a different language
		"node_modules/x": "go.mod",         // pruned: dependency tree
		"vendor/dep":     "go.mod",         // pruned: vendored deps
		".git":           "go.mod",         // pruned: hidden/VCS
		"a/b/c":          "go.mod",         // depth 3, at the default limit
		"a/b/c/d":        "go.mod",         // depth 4, beyond the default limit
	})

	defer func(l []string, d int) { languages, maxDepth = l, d }(languages, maxDepth)
	languages, maxDepth = nil, defaultMaxDepth

	dirs, err := collectRepos([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	got := relSet(t, root, dirs)
	want := []string{"a/b/c", "svc-go", "svc-go/backend"}
	if !equalSet(got, want) {
		t.Errorf("collectRepos = %v, want %v", got, want)
	}
}

func TestCollectReposDepthFlag(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, map[string]string{
		"a/b/c/d": "go.mod", // depth 4
	})

	defer func(l []string, d int) { languages, maxDepth = l, d }(languages, maxDepth)
	languages = nil

	maxDepth = defaultMaxDepth // 3: too shallow to reach depth 4
	dirs, _ := collectRepos([]string{root})
	if len(dirs) != 0 {
		t.Errorf("depth 3: found %v, want none", relSet(t, root, dirs))
	}

	maxDepth = 4 // now deep enough
	dirs, _ = collectRepos([]string{root})
	if got := relSet(t, root, dirs); !equalSet(got, []string{"a/b/c/d"}) {
		t.Errorf("depth 4: found %v, want [a/b/c/d]", got)
	}
}
