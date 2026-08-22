package recipe_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/recipe"
	"github.com/whoisnjoguu/elostirion/pkg/spec"

	// Register the version editors.
	_ "github.com/whoisnjoguu/elostirion/pkg/scan/gomod"
	_ "github.com/whoisnjoguu/elostirion/pkg/scan/python"
)

func TestBumpLanguageVersionGo(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/svc\n\ngo 1.24.6\n")},
	}
	r, ok := recipe.Get("bump-language-version")
	if !ok {
		t.Fatal("bump-language-version not registered")
	}
	rule := spec.Rule{ID: "go-min", Scanner: "gomod", Field: "go_version", Op: spec.OpGTE, Value: "1.25.12", Recipe: "bump-language-version"}
	edits, err := r.Apply(fsys, rule, model.Finding{})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || edits[0].Path != "go.mod" {
		t.Fatalf("edits=%+v", edits)
	}
	if !strings.Contains(string(edits[0].Content), "go 1.25.12") {
		t.Errorf("go.mod not bumped:\n%s", edits[0].Content)
	}
}

func TestBumpLanguageVersionGoNoop(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/svc\n\ngo 1.25.12\n")},
	}
	r, _ := recipe.Get("bump-language-version")
	rule := spec.Rule{ID: "go-min", Scanner: "gomod", Op: spec.OpGTE, Value: "1.25.12"}
	edits, err := r.Apply(fsys, rule, model.Finding{})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 0 {
		t.Fatalf("expected no edits, got %+v", edits)
	}
}

func TestBumpLanguageVersionPython(t *testing.T) {
	fsys := fstest.MapFS{
		"pyproject.toml": &fstest.MapFile{Data: []byte("[project]\nname = \"svc\"\nrequires-python = \">=3.10\"\n")},
	}
	r, _ := recipe.Get("bump-language-version")
	rule := spec.Rule{ID: "py-min", Scanner: "python", Op: spec.OpGTE, Target: ">=3.12"}
	edits, err := r.Apply(fsys, rule, model.Finding{})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || !strings.Contains(string(edits[0].Content), `requires-python = ">=3.12"`) {
		t.Fatalf("pyproject not bumped: %+v", edits)
	}
}

func TestBumpBaseImage(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte("FROM golang:1.25 AS builder\nRUN go build\n\nFROM alpine:latest\nCOPY --from=builder /app /app\n")},
	}
	r, ok := recipe.Get("bump-base-image")
	if !ok {
		t.Fatal("bump-base-image not registered")
	}
	rule := spec.Rule{ID: "img", Scanner: "dockerfile", Op: spec.OpMatches, Value: "^alpine:3", Target: "alpine:3.22", Recipe: "bump-base-image"}
	finding := model.Finding{Location: model.Location{File: "Dockerfile", Line: 4}}
	edits, err := r.Apply(fsys, rule, finding)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 {
		t.Fatalf("edits=%+v", edits)
	}
	out := string(edits[0].Content)
	if !strings.Contains(out, "FROM alpine:3.22") {
		t.Errorf("final image not rewritten:\n%s", out)
	}
	if !strings.Contains(out, "FROM golang:1.25 AS builder") {
		t.Errorf("builder stage should be untouched:\n%s", out)
	}
}

func TestTargetRequired(t *testing.T) {
	rule := spec.Rule{ID: "x", Op: spec.OpMatches, Value: "^alpine"}
	if _, err := recipe.Target(rule); err == nil {
		t.Fatal("expected error for pattern rule without target")
	}
	rule.Target = "alpine:3.22"
	got, err := recipe.Target(rule)
	if err != nil || got != "alpine:3.22" {
		t.Fatalf("got %q err %v", got, err)
	}
}
