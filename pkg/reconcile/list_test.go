package reconcile

import (
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// listFacts builds facts with a pipeline.steps and pipeline.images list.
func listFacts() *model.Facts {
	f := model.NewFacts(model.Repo{Name: "svc"})
	f.SetList("pipeline.steps", []string{"lint", "test", "build"}, []model.Location{
		{File: "bitbucket-pipelines.yml", Line: 5},
		{File: "bitbucket-pipelines.yml", Line: 9},
		{File: "bitbucket-pipelines.yml", Line: 13},
	})
	f.SetList("pipeline.images", []string{"golang:1.25", "postgres:16"}, []model.Location{
		{File: "bitbucket-pipelines.yml", Line: 1},
		{File: "bitbucket-pipelines.yml", Line: 15},
	})
	return f
}

func evalOne(t *testing.T, rule spec.Rule, facts *model.Facts) []model.Finding {
	t.Helper()
	rule.Severity = model.SeverityError
	if rule.ID == "" {
		rule.ID = "r"
	}
	return Evaluate(&spec.Spec{Version: 1, Rules: []spec.Rule{rule}}, facts)
}

func TestListContainsMembership(t *testing.T) {
	facts := listFacts()

	ok := evalOne(t, spec.Rule{Scanner: "pipeline", Field: "steps", Op: spec.OpContains, Value: "test"}, facts)
	if len(ok) != 0 {
		t.Errorf("contains test: want conformant, got %+v", ok)
	}

	missing := evalOne(t, spec.Rule{Scanner: "pipeline", Field: "steps", Op: spec.OpContains, Value: "security-scan"}, facts)
	if len(missing) != 1 {
		t.Fatalf("contains security-scan: want 1 finding, got %d", len(missing))
	}
	if missing[0].Got != "lint, test, build" || missing[0].Want != "security-scan" {
		t.Errorf("got=%q want=%q", missing[0].Got, missing[0].Want)
	}
}

func TestListMatchesEveryElement(t *testing.T) {
	facts := listFacts()

	// Every image from an approved registry? postgres:16 violates and the
	// finding points at its element location.
	f := evalOne(t, spec.Rule{Scanner: "pipeline", Field: "images", Op: spec.OpMatches, Value: `^golang:`}, facts)
	if len(f) != 1 {
		t.Fatalf("want 1 finding, got %d", len(f))
	}
	if f[0].Got != "postgres:16" {
		t.Errorf("Got=%q want postgres:16 (the offending element)", f[0].Got)
	}
	if f[0].Location.Line != 15 {
		t.Errorf("Location=%+v want line 15 (element location)", f[0].Location)
	}
}

func TestListNotMatches(t *testing.T) {
	facts := listFacts()
	f := evalOne(t, spec.Rule{Scanner: "pipeline", Field: "images", Op: spec.OpNotMatches, Value: `:latest$`}, facts)
	if len(f) != 0 {
		t.Errorf("no :latest images: want conformant, got %+v", f)
	}

	facts.SetList("pipeline.images", []string{"golang:1.25", "redis:latest"}, []model.Location{
		{File: "f", Line: 1}, {File: "f", Line: 2},
	})
	f = evalOne(t, spec.Rule{Scanner: "pipeline", Field: "images", Op: spec.OpNotMatches, Value: `:latest$`}, facts)
	if len(f) != 1 || f[0].Got != "redis:latest" {
		t.Errorf("want redis:latest flagged, got %+v", f)
	}
}

func TestListOneOfEveryElement(t *testing.T) {
	facts := listFacts()
	f := evalOne(t, spec.Rule{
		Scanner: "pipeline", Field: "images", Op: spec.OpOneOf,
		Values: []string{"golang:1.25", "postgres:16", "redis:7"},
	}, facts)
	if len(f) != 0 {
		t.Errorf("all images allowed: want conformant, got %+v", f)
	}

	f = evalOne(t, spec.Rule{
		Scanner: "pipeline", Field: "images", Op: spec.OpOneOf,
		Values: []string{"golang:1.25"},
	}, facts)
	if len(f) != 1 || f[0].Got != "postgres:16" {
		t.Errorf("want postgres:16 flagged, got %+v", f)
	}
}

func TestListAbsentFact(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	f := evalOne(t, spec.Rule{Scanner: "pipeline", Field: "steps", Op: spec.OpContains, Value: "test"}, facts)
	if len(f) != 1 || f[0].Got != "<missing>" {
		t.Errorf("missing list fact: want <missing> violation, got %+v", f)
	}
}

func TestSequenceOrdered(t *testing.T) {
	facts := listFacts() // steps: lint, test, build

	// Subsequence with interleaved extras is conformant.
	f := evalOne(t, spec.Rule{
		Scanner: "pipeline", Field: "steps", Op: spec.OpSequence,
		Values: []string{"lint", "build"},
	}, facts)
	if len(f) != 0 {
		t.Errorf("lint→build subsequence: want conformant, got %+v", f)
	}

	// Out of order violates.
	f = evalOne(t, spec.Rule{
		Scanner: "pipeline", Field: "steps", Op: spec.OpSequence,
		Values: []string{"test", "lint"},
	}, facts)
	if len(f) != 1 {
		t.Fatalf("test before lint: want violation, got none")
	}
	if f[0].Got != "lint, test, build" || f[0].Want != "test, lint" {
		t.Errorf("got=%q want=%q", f[0].Got, f[0].Want)
	}

	// Missing element violates.
	f = evalOne(t, spec.Rule{
		Scanner: "pipeline", Field: "steps", Op: spec.OpSequence,
		Values: []string{"lint", "deploy"},
	}, facts)
	if len(f) != 1 {
		t.Errorf("missing deploy: want violation, got none")
	}
}

func TestSequenceOnScalarFactViolates(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("gomod.go_version", "1.25", model.Location{File: "go.mod"})
	f := evalOne(t, spec.Rule{
		Scanner: "gomod", Field: "go_version", Op: spec.OpSequence,
		Values: []string{"a", "b"},
	}, facts)
	if len(f) != 1 {
		t.Errorf("sequence on scalar: want violation (unsupported), got %+v", f)
	}
}
