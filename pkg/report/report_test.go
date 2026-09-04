package report

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

var update = flag.Bool("update", false, "update golden files")

// fixtureReport is the canonical report the golden tests render
func fixtureReport() *Report {
	return &Report{
		SpecName: "enterprise-apis-fleet",
		Results: []RepoResult{
			{Repo: model.Repo{Provider: "github", Owner: "acme", Name: "api"}},
			{
				Repo: model.Repo{Provider: "github", Owner: "acme", Name: "web"},
				Findings: []model.Finding{
					{
						RuleID:   "go-version-min",
						Message:  "Go version in go.mod must be at least 1.25.12.",
						Severity: model.SeverityError,
						Repo:     model.Repo{Owner: "acme", Name: "web"},
						Location: model.Location{File: "go.mod", Line: 5},
						Got:      "1.22", Want: "1.25.12",
					},
					{
						RuleID:   "base-image-approved",
						Message:  "Dockerfile base image must be an approved golang tag.",
						Severity: model.SeverityWarn,
						Repo:     model.Repo{Owner: "acme", Name: "web"},
						Location: model.Location{File: "Dockerfile"},
						Got:      "golang:1.22", Want: "golang:1.25",
					},
					{
						RuleID:   "tf-module-pinned",
						Message:  "Terraform module drifted from state.",
						Severity: model.SeverityDrift,
						Repo:     model.Repo{Owner: "acme", Name: "web"},
						Got:      "1.0.0", Want: "1.2.0",
					},
					{
						RuleID:   "project-name-set",
						Message:  "pyproject.toml should declare a project name.",
						Severity: model.SeverityInfo,
						Repo:     model.Repo{Owner: "acme", Name: "web"},
						Location: model.Location{File: "pyproject.toml", Line: 2},
					},
				},
			},
		},
	}
}

func TestRenderersGolden(t *testing.T) {
	cases := []struct {
		name     string
		golden   string
		renderer Renderer
	}{
		{"text-color", "text-color.golden", textRenderer{color: true}},
		{"text-plain", "text-plain.golden", textRenderer{color: false}},
		{"json", "report.json.golden", jsonRenderer{}},
		{"junit", "report.junit.golden", junitRenderer{}},
		{"sarif", "report.sarif.golden", sarifRenderer{}},
	}

	rep := fixtureReport()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.renderer.Render(rep)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			assertGolden(t, tc.golden, got)
		})
	}
}

// assertGolden compares got against the golden file, writing it when -update is set.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./pkg/report -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output does not match %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
