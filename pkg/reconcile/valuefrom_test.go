package reconcile

import (
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// crossRule asserts the Dockerfile builder image tracks the go.mod version.
func crossRule() spec.Rule {
	return spec.Rule{
		ID: "builder-tracks-gomod", Severity: model.SeverityError,
		Scanner: "dockerfile", Field: "builder_image", Op: spec.OpMatches,
		ValueFrom: &spec.ValueFrom{Key: "gomod.go_version", Template: `^golang:{value.major_minor}`},
	}
}

func TestValueFromCrossFactViolation(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("gomod.go_version", "1.25.1", model.Location{File: "go.mod", Line: 3})
	facts.Set("dockerfile.builder_image", "golang:1.22", model.Location{File: "Dockerfile", Line: 1})

	f := Evaluate(&spec.Spec{Version: 1, Rules: []spec.Rule{crossRule()}}, facts)
	if len(f) != 1 {
		t.Fatalf("want 1 finding, got %+v", f)
	}
	if f[0].Got != "golang:1.22" {
		t.Errorf("Got=%q want golang:1.22", f[0].Got)
	}
	if f[0].Want != "^golang:1.25" {
		t.Errorf("Want=%q want ^golang:1.25 (derived from go.mod)", f[0].Want)
	}
	if f[0].Location.File != "Dockerfile" {
		t.Errorf("Location=%+v want Dockerfile", f[0].Location)
	}
}

func TestValueFromCrossFactConformant(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("gomod.go_version", "1.25.1", model.Location{File: "go.mod"})
	facts.Set("dockerfile.builder_image", "golang:1.25-alpine", model.Location{File: "Dockerfile"})

	if f := Evaluate(&spec.Spec{Version: 1, Rules: []spec.Rule{crossRule()}}, facts); len(f) != 0 {
		t.Errorf("aligned versions: want conformant, got %+v", f)
	}
}

func TestValueFromSkippedWhenRefAbsent(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("dockerfile.builder_image", "golang:1.22", model.Location{File: "Dockerfile"})
	// no gomod facts: not a Go repo, rule cannot assert

	if f := Evaluate(&spec.Spec{Version: 1, Rules: []spec.Rule{crossRule()}}, facts); len(f) != 0 {
		t.Errorf("absent reference fact: want rule skipped, got %+v", f)
	}
}

func TestValueFromDefaultTemplateEq(t *testing.T) {
	r := spec.Rule{
		ID: "images-match", Severity: model.SeverityError,
		Scanner: "dockerfile", Field: "base_image", Op: spec.OpEquals,
		ValueFrom: &spec.ValueFrom{Key: "dockerfile.builder_image"},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("dockerfile.builder_image", "golang:1.25", model.Location{File: "Dockerfile", Line: 1})
	facts.Set("dockerfile.base_image", "gcr.io/distroless/base", model.Location{File: "Dockerfile", Line: 9})

	f := Evaluate(&spec.Spec{Version: 1, Rules: []spec.Rule{r}}, facts)
	if len(f) != 1 || f[0].Want != "golang:1.25" {
		t.Errorf("default template: want derived golang:1.25, got %+v", f)
	}
}

func TestMajorMinor(t *testing.T) {
	cases := map[string]string{"1.25.1": "1.25", "1.25": "1.25", "v1.9.0": "1.9", "22": "22"}
	for in, want := range cases {
		if got := majorMinor(in); got != want {
			t.Errorf("majorMinor(%q)=%q want %q", in, got, want)
		}
	}
}
