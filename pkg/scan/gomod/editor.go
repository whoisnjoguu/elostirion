// Package gomod scans and edits go.mod files.
package gomod

import (
	"fmt"
	"io"
	"io/fs"

	"golang.org/x/mod/modfile"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/recipe"
)

func init() { recipe.RegisterVersionEditor(Editor{}) }

// Editor rewrites the go directive in go.mod for bump-language-version.
type Editor struct{}

// Scanner pairs the editor with the gomod scanner.
func (Editor) Scanner() string { return "gomod" }

// BumpVersion sets the go directive to target and returns the edited go.mod.
func (Editor) BumpVersion(fsys fs.FS, target string) ([]model.FileEdit, error) {
	f, err := fsys.Open("go.mod")
	if err != nil {
		return nil, fmt.Errorf("bump-language-version: no go.mod: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("bump-language-version: parse go.mod: %w", err)
	}
	if mf.Go != nil && mf.Go.Version == target {
		return nil, nil
	}
	if err := mf.AddGoStmt(target); err != nil {
		return nil, err
	}
	out, err := mf.Format()
	if err != nil {
		return nil, err
	}
	return []model.FileEdit{{Path: "go.mod", Content: out}}, nil
}
