package diff

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func TestUnified(t *testing.T) {
	out := Unified("go.mod", []byte("module m\n\ngo 1.24.6\n"), []byte("module m\n\ngo 1.25.12\n"))
	if !strings.Contains(out, "-go 1.24.6") || !strings.Contains(out, "+go 1.25.12") {
		t.Errorf("unexpected diff:\n%s", out)
	}
	if !strings.Contains(out, "a/go.mod") || !strings.Contains(out, "b/go.mod") {
		t.Errorf("missing file headers:\n%s", out)
	}
}

func TestForPlan(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("go 1.24\n")},
	}
	plan := model.ChangePlan{Edits: []model.FileEdit{{Path: "go.mod", Content: []byte("go 1.25\n")}}}
	out := ForPlan(fsys, plan)
	if !strings.Contains(out, "-go 1.24") || !strings.Contains(out, "+go 1.25") {
		t.Errorf("unexpected plan diff:\n%s", out)
	}
}

func TestColorize(t *testing.T) {
	in := "--- a/go.mod\n+++ b/go.mod\n@@ -1 +1 @@\n-go 1.24\n+go 1.25\n context\n"
	out := Colorize(in)
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI sequences in colorized diff:\n%q", out)
	}
	if !strings.Contains(out, "go 1.24") || !strings.Contains(out, "go 1.25") {
		t.Errorf("original text lost:\n%q", out)
	}
	if !strings.Contains(out, "\n context\n") {
		t.Errorf("context line should be unstyled:\n%q", out)
	}
}
