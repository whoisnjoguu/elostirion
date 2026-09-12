package cli

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/diff"
	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// TestBuildPlanFSSourceAgnostic proves the plan builder runs recipes over any
// fs.FS, so remote plan produces the same edits a local checkout would.
func TestBuildPlanFSSourceAgnostic(t *testing.T) {
	s := &spec.Spec{Version: 1, Rules: []spec.Rule{{
		ID: "lint-config", File: ".golangci.yml", Op: spec.OpEquals,
		Severity: model.SeverityWarn, Recipe: "sync-file",
		Value: "run:\n  timeout: 5m\n",
	}}}
	fsys := fstest.MapFS{".golangci.yml": &fstest.MapFile{Data: []byte("run:\n  timeout: 10m\n")}}
	repo := model.Repo{Provider: "github", Owner: "acme", Name: "api"}

	findings := []model.Finding{{RuleID: "lint-config"}}
	plan := buildPlanFS(s, fsys, repo, findings)

	if len(plan.Edits) != 1 || plan.Edits[0].Path != ".golangci.yml" {
		t.Fatalf("edits=%+v want one edit to .golangci.yml", plan.Edits)
	}
	if got := string(plan.Edits[0].Content); !strings.Contains(got, "timeout: 5m") {
		t.Errorf("edit content=%q want converged template", got)
	}
	if plan.Recipe != "sync-file" {
		t.Errorf("recipe=%q want sync-file", plan.Recipe)
	}

	// The remote plan path renders the diff with diff.ForPlan over the same
	// (non-local) fs.FS, so prove that produces a unified diff here.
	d := diff.ForPlan(fsys, plan)
	if !strings.Contains(d, "-  timeout: 10m") || !strings.Contains(d, "+  timeout: 5m") {
		t.Errorf("diff over in-memory fs did not render the change:\n%s", d)
	}
}

// TestBuildPlanFSNoRecipe leaves a violation without a recipe unplanned.
func TestBuildPlanFSNoRecipe(t *testing.T) {
	s := &spec.Spec{Version: 1, Rules: []spec.Rule{{
		ID: "codeowners", File: "CODEOWNERS", Op: spec.OpExists, Severity: model.SeverityError,
	}}}
	plan := buildPlanFS(s, fstest.MapFS{}, model.Repo{Name: "api"}, []model.Finding{{RuleID: "codeowners"}})
	if !plan.Empty() {
		t.Errorf("no recipe: want empty plan, got %+v", plan.Edits)
	}
}
