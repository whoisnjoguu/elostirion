// Package recipe turns findings into concrete file edits
package recipe

import (
	"fmt"
	"io/fs"
	"sort"
	"sync"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// Recipe converts one violated rule into file edits that fix it
type Recipe interface {
	Name() string // identifier referenced by a rule's recipe
	Description() string
	Apply(fsys fs.FS, rule spec.Rule, finding model.Finding) ([]model.FileEdit, error) // compute edits that converge fsys to the rule's target
}

var (
	mu      sync.RWMutex
	recipes = map[string]Recipe{}
)

// Register makes a recipe available by name.
func Register(r Recipe) {
	mu.Lock()
	defer mu.Unlock()
	recipes[r.Name()] = r
}

// Get returns a registered recipe by name.
func Get(name string) (Recipe, bool) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := recipes[name]
	return r, ok
}

// Names returns the registered recipe names, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(recipes))
	for n := range recipes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Target resolves the concrete value a recipe should converge to
func Target(rule spec.Rule) (string, error) {
	if rule.Target != "" {
		return rule.Target, nil
	}
	switch rule.Op {
	case spec.OpEquals, spec.OpGTE, spec.OpLTE:
		if rule.Value != "" {
			return rule.Value, nil
		}
	}
	return "", fmt.Errorf("recipe: rule %q has no usable target; set target: to the concrete value to converge to", rule.ID)
}

// VersionEditor rewrites the language-version declaration of one ecosystem's manifest
type VersionEditor interface {
	Scanner() string
	BumpVersion(fsys fs.FS, target string) ([]model.FileEdit, error)
}

var (
	emu     sync.RWMutex
	editors = map[string]VersionEditor{}
)

// RegisterVersionEditor makes an editor available to bump-language-version.
func RegisterVersionEditor(e VersionEditor) {
	emu.Lock()
	defer emu.Unlock()
	editors[e.Scanner()] = e
}

// editorFor returns the version editor paired with a scanner.
func editorFor(scanner string) (VersionEditor, bool) {
	emu.RLock()
	defer emu.RUnlock()
	e, ok := editors[scanner]
	return e, ok
}
