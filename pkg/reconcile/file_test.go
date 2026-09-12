package reconcile

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

func fileSpec(rules ...spec.Rule) *spec.Spec {
	for i := range rules {
		if rules[i].ID == "" {
			rules[i].ID = "r" + string(rune('0'+i))
		}
		rules[i].Severity = model.SeverityError
	}
	return &spec.Spec{Version: 1, Rules: rules}
}

func TestFileRuleExists(t *testing.T) {
	s := fileSpec(spec.Rule{ID: "license", File: "LICENSE", Op: spec.OpExists})

	fsys := fstest.MapFS{"LICENSE": &fstest.MapFile{Data: []byte("MIT")}}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if f := EvaluateFS(s, facts, fsys); len(f) != 0 {
		t.Errorf("LICENSE present: want conformant, got %+v", f)
	}

	facts = model.NewFacts(model.Repo{Name: "svc"})
	if f := EvaluateFS(s, facts, fstest.MapFS{}); len(f) != 1 {
		t.Errorf("LICENSE missing: want 1 finding, got %+v", f)
	}
}

func TestFileRuleAbsent(t *testing.T) {
	s := fileSpec(spec.Rule{ID: "no-env", File: ".env", Op: spec.OpAbsent})
	fsys := fstest.MapFS{".env": &fstest.MapFile{Data: []byte("SECRET=1")}}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	f := EvaluateFS(s, facts, fsys)
	if len(f) != 1 {
		t.Fatalf(".env present: want 1 finding, got %+v", f)
	}
	if f[0].Location.File != ".env" {
		t.Errorf("location=%+v want .env", f[0].Location)
	}
}

func TestFileRuleMatches(t *testing.T) {
	s := fileSpec(spec.Rule{ID: "codeowners", File: ".github/CODEOWNERS", Op: spec.OpMatches, Value: `(?m)^\*\s+@acme/platform$`})
	fsys := fstest.MapFS{".github/CODEOWNERS": &fstest.MapFile{Data: []byte("* @acme/platform\n")}}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if f := EvaluateFS(s, facts, fsys); len(f) != 0 {
		t.Errorf("CODEOWNERS ok: want conformant, got %+v", f)
	}

	fsys = fstest.MapFS{".github/CODEOWNERS": &fstest.MapFile{Data: []byte("* @acme/other\n")}}
	facts = model.NewFacts(model.Repo{Name: "svc"})
	if f := EvaluateFS(s, facts, fsys); len(f) != 1 {
		t.Errorf("CODEOWNERS wrong: want 1 finding, got %+v", f)
	}
}

func TestFileRuleTemplateEq(t *testing.T) {
	template := "run:\n  timeout: 5m\n"
	s := fileSpec(spec.Rule{ID: "lint-config", File: ".golangci.yml", Op: spec.OpEquals, Value: template})

	// Trailing whitespace and a missing final newline must not defeat equality.
	fsys := fstest.MapFS{".golangci.yml": &fstest.MapFile{Data: []byte("run:  \n  timeout: 5m")}}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if f := EvaluateFS(s, facts, fsys); len(f) != 0 {
		t.Errorf("template match modulo whitespace: want conformant, got %+v", f)
	}

	fsys = fstest.MapFS{".golangci.yml": &fstest.MapFile{Data: []byte("run:\n  timeout: 10m\n")}}
	facts = model.NewFacts(model.Repo{Name: "svc"})
	f := EvaluateFS(s, facts, fsys)
	if len(f) != 1 {
		t.Fatalf("template mismatch: want 1 finding, got %+v", f)
	}
	// Multiline got/want are trimmed to a single display line.
	if strings.Contains(f[0].Got, "\n") || strings.Contains(f[0].Want, "\n") {
		t.Errorf("got/want should be single-line: %q / %q", f[0].Got, f[0].Want)
	}
}

func TestFileRuleMissingFileNonPresenceOp(t *testing.T) {
	s := fileSpec(spec.Rule{ID: "codeowners", File: "CODEOWNERS", Op: spec.OpMatches, Value: `platform`})
	facts := model.NewFacts(model.Repo{Name: "svc"})
	f := EvaluateFS(s, facts, fstest.MapFS{})
	if len(f) != 1 || f[0].Got != "<missing>" {
		t.Errorf("missing file with matches op: want <missing> violation, got %+v", f)
	}
}
