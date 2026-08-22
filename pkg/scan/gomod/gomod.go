package gomod

import (
	"io"
	"io/fs"

	"golang.org/x/mod/modfile"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

func init() { scan.Register(Scanner{}) }

// Scanner extracts facts from a repository's go.mod
type Scanner struct{}

// Name returns the fact namespace
func (Scanner) Name() string { return "gomod" }

// Language reports the language this scanner belongs to
func (Scanner) Language() string { return "go" }

// Markers are files that identify a Go repository.
func (Scanner) Markers() []string { return []string{"go.mod"} }

// Scan parses go.mod and records module facts
func (Scanner) Scan(fsys fs.FS, facts *model.Facts) error {
	f, err := fsys.Open("go.mod")
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return err
	}

	loc := model.Location{File: "go.mod"}
	if mf.Module != nil {
		facts.Set("gomod.module", mf.Module.Mod.Path, loc)
	}
	if mf.Go != nil {
		facts.Set("gomod.go_version", mf.Go.Version, loc)
	}
	facts.Set("gomod.require_count", len(mf.Require), loc)
	return nil
}
