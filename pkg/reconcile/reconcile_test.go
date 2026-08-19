package reconcile

import (
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.24", "1.24", 0},
		{"1.24", "1.23", 1},
		{"1.23", "1.24", -1},
		{"1.24.3", "1.24", 1},
		{"v1.25", "1.24", 1},
		{"1.9", "1.10", -1},
	}
	for _, c := range cases {
		got, err := compareVersions(c.a, c.b)
		if err != nil {
			t.Fatalf("compareVersions(%q,%q): %v", c.a, c.b, err)
		}
		if got != c.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	s := &spec.Spec{
		Version: 1,
		Rules: []spec.Rule{
			{ID: "go-min", Severity: model.SeverityError, Scanner: "gomod", Field: "go_version", Op: spec.OpGTE, Value: "1.24"},
			{ID: "df-exists", Severity: model.SeverityError, Scanner: "dockerfile", Field: "base_image", Op: spec.OpExists},
		},
	}

	facts := model.NewFacts(model.Repo{Name: "svc"})
	facts.Set("gomod.go_version", "1.23", model.Location{File: "go.mod"})
	// dockerfile.base_image intentionally absent

	findings := Evaluate(s, facts)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}

	facts2 := model.NewFacts(model.Repo{Name: "svc"})
	facts2.Set("gomod.go_version", "1.25", model.Location{File: "go.mod"})
	facts2.Set("dockerfile.base_image", "golang:1.25", model.Location{File: "Dockerfile"})
	if got := Evaluate(s, facts2); len(got) != 0 {
		t.Fatalf("expected conformant, got findings: %+v", got)
	}
}

func TestOneOfAndMatches(t *testing.T) {
	s := &spec.Spec{
		Version: 1,
		Rules: []spec.Rule{
			{ID: "oneof", Severity: model.SeverityWarn, Scanner: "x", Field: "v", Op: spec.OpOneOf, Values: []string{"a", "b"}},
			{ID: "match", Severity: model.SeverityWarn, Scanner: "x", Field: "img", Op: spec.OpMatches, Value: "^golang:"},
		},
	}
	f := model.NewFacts(model.Repo{Name: "svc"})
	f.Set("x.v", "c", model.Location{})
	f.Set("x.img", "alpine", model.Location{})
	if got := Evaluate(s, f); len(got) != 2 {
		t.Fatalf("want 2 findings, got %d", len(got))
	}
}
