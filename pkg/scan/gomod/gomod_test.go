package gomod

import (
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func TestScan(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/svc\n\ngo 1.24\n\nrequire github.com/spf13/cobra v1.10.2\n")},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	if v, _ := facts.Get("gomod.go_version"); v != "1.24" {
		t.Errorf("go_version=%v want 1.24", v)
	}
	if m, _ := facts.Get("gomod.module"); m != "example.com/svc" {
		t.Errorf("module=%v want example.com/svc", m)
	}
	if rc, _ := facts.Get("gomod.require_count"); rc != 1 {
		t.Errorf("require_count=%v want 1", rc)
	}
}

func TestScanNoGoMod(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fstest.MapFS{}, facts); err != nil {
		t.Fatal(err)
	}
	if _, ok := facts.Get("gomod.go_version"); ok {
		t.Error("go_version should be absent without go.mod")
	}
}
