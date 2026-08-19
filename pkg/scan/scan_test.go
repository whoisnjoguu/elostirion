package scan_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

// twoLangScanner is a test scanner tagged with a language.
type langScanner struct {
	name, lang string
	marker     string
}

func (s langScanner) Name() string      { return s.name }
func (s langScanner) Language() string  { return s.lang }
func (s langScanner) Markers() []string { return []string{s.marker} }
func (s langScanner) Scan(_ fs.FS, f *model.Facts) error {
	f.Set(s.name+".ran", true, model.Location{})
	return nil
}

// agnosticScanner has no language and should always run.
type agnosticScanner struct{}

func (agnosticScanner) Name() string { return "agnostic" }
func (agnosticScanner) Scan(_ fs.FS, f *model.Facts) error {
	f.Set("agnostic.ran", true, model.Location{})
	return nil
}

func TestLanguageSelection(t *testing.T) {
	scan.Register(langScanner{name: "goish", lang: "go", marker: "go.mod"})
	scan.Register(langScanner{name: "pyish", lang: "py", marker: "pyproject.toml"})
	scan.Register(agnosticScanner{})

	fsys := fstest.MapFS{}
	repo := model.Repo{Name: "svc"}

	// Selecting go runs the go scanner and the agnostic one, not python.
	facts, err := scan.Run(fsys, repo, "go")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := facts.Get("goish.ran"); !ok {
		t.Error("go scanner should run for --language go")
	}
	if _, ok := facts.Get("agnostic.ran"); !ok {
		t.Error("agnostic scanner should always run")
	}
	if _, ok := facts.Get("pyish.ran"); ok {
		t.Error("python scanner should not run for --language go")
	}

	// Markers for go should include go.mod but not pyproject.toml.
	markers := scan.MarkersFor("go")
	if !contains(markers, "go.mod") || contains(markers, "pyproject.toml") {
		t.Errorf("MarkersFor(go)=%v", markers)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
