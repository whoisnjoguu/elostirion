package recipe

import (
	"strings"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// syncRule builds the canonical sync-pipeline rule: steps must contain "security-scan".
func syncRule() spec.Rule {
	return spec.Rule{
		ID: "pipeline-security-scan", Scanner: "pipeline", Field: "steps",
		Op: spec.OpContains, Value: "security-scan", Recipe: "sync-pipeline",
		Target: "script:\n  - ./scripts/security-scan.sh\n",
	}
}

func applySync(t *testing.T, file, content string) string {
	t.Helper()
	fsys := fstest.MapFS{file: &fstest.MapFile{Data: []byte(content)}}
	finding := model.Finding{RuleID: "pipeline-security-scan", Location: model.Location{File: file}}
	edits, err := syncPipeline{}.Apply(fsys, syncRule(), finding)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || edits[0].Path != file {
		t.Fatalf("edits=%+v want one edit to %s", edits, file)
	}
	return string(edits[0].Content)
}

func TestSyncPipelineBitbucket(t *testing.T) {
	out := applySync(t, "bitbucket-pipelines.yml", `image: golang:1.25
pipelines:
  default:
    - step:
        name: build
        script:
          - go build ./...
`)
	var doc struct {
		Pipelines struct {
			Default []map[string]struct {
				Name   string   `yaml:"name"`
				Script []string `yaml:"script"`
			} `yaml:"default"`
		} `yaml:"pipelines"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, out)
	}
	steps := doc.Pipelines.Default
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2\n%s", len(steps), out)
	}
	last := steps[1]["step"]
	if last.Name != "security-scan" || len(last.Script) != 1 || last.Script[0] != "./scripts/security-scan.sh" {
		t.Errorf("appended step = %+v, want security-scan with script", last)
	}
}

func TestSyncPipelineGitHub(t *testing.T) {
	out := applySync(t, ".github/workflows/ci.yml", `name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`)
	var doc struct {
		Jobs map[string]any `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, out)
	}
	if _, ok := doc.Jobs["security-scan"]; !ok {
		t.Errorf("security-scan job not added:\n%s", out)
	}
	if _, ok := doc.Jobs["test"]; !ok {
		t.Errorf("existing test job lost:\n%s", out)
	}
}

func TestSyncPipelineGitLab(t *testing.T) {
	out := applySync(t, ".gitlab-ci.yml", `image: golang:1.25
lint:
  script:
    - golangci-lint run
`)
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, out)
	}
	if _, ok := doc["security-scan"]; !ok {
		t.Errorf("security-scan job not added:\n%s", out)
	}
	if _, ok := doc["lint"]; !ok {
		t.Errorf("existing lint job lost:\n%s", out)
	}
}

func TestSyncPipelineNoFile(t *testing.T) {
	finding := model.Finding{RuleID: "x"} // no location: repo has no pipeline file
	_, err := syncPipeline{}.Apply(fstest.MapFS{}, syncRule(), finding)
	if err == nil || !strings.Contains(err.Error(), "no pipeline file") {
		t.Errorf("err=%v, want no-pipeline-file error", err)
	}
}

func TestSyncPipelineNeedsTarget(t *testing.T) {
	r := syncRule()
	r.Target = ""
	finding := model.Finding{Location: model.Location{File: ".gitlab-ci.yml"}}
	_, err := syncPipeline{}.Apply(fstest.MapFS{}, r, finding)
	if err == nil || !strings.Contains(err.Error(), "target") {
		t.Errorf("err=%v, want missing-target error", err)
	}
}
