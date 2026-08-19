package python

import (
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func TestScanPyproject(t *testing.T) {
	fsys := fstest.MapFS{
		"pyproject.toml": &fstest.MapFile{Data: []byte(`[project]
name = "svc"
requires-python = ">=3.11"
`)},
		"poetry.lock": &fstest.MapFile{Data: []byte("")},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	if v, _ := facts.Get("python.requires_version"); v != ">=3.11" {
		t.Errorf("requires_version=%v want >=3.11", v)
	}
	if v, _ := facts.Get("python.project_name"); v != "svc" {
		t.Errorf("project_name=%v want svc", v)
	}
	if v, _ := facts.Get("python.package_manager"); v != "poetry" {
		t.Errorf("package_manager=%v want poetry", v)
	}
}

func TestScanRequirementsOnly(t *testing.T) {
	fsys := fstest.MapFS{
		"requirements.txt": &fstest.MapFile{Data: []byte("flask==3.0\n")},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	if v, _ := facts.Get("python.package_manager"); v != "pip" {
		t.Errorf("package_manager=%v want pip", v)
	}
}

func TestMeta(t *testing.T) {
	if (Scanner{}).Language() != "py" {
		t.Errorf("language=%q want py", (Scanner{}).Language())
	}
	if len((Scanner{}).Markers()) == 0 {
		t.Error("expected markers")
	}
}
