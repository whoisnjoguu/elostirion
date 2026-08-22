// Package python scans and edits Python project files.
package python

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/recipe"
)

func init() { recipe.RegisterVersionEditor(Editor{}) }

// Editor rewrites requires-python in pyproject.toml for bump-language-version.
type Editor struct{}

// Scanner pairs the editor with the python scanner.
func (Editor) Scanner() string { return "python" }

// BumpVersion sets requires-python to target and returns the edited pyproject.toml
func (Editor) BumpVersion(fsys fs.FS, target string) ([]model.FileEdit, error) {
	data, ok := readFile(fsys, "pyproject.toml")
	if !ok {
		return nil, fmt.Errorf("bump-language-version: no pyproject.toml")
	}
	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "requires-python") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "requires-python"))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		if current := strings.Trim(strings.TrimSpace(strings.TrimPrefix(rest, "=")), `"'`); current == target {
			return nil, nil
		}
		lines[i] = fmt.Sprintf(`requires-python = %q`, target)
		replaced = true
		break
	}
	if !replaced {
		return nil, fmt.Errorf("bump-language-version: pyproject.toml has no requires-python to rewrite")
	}
	return []model.FileEdit{{Path: "pyproject.toml", Content: []byte(strings.Join(lines, "\n"))}}, nil
}
