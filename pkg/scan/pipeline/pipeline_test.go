package pipeline

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

const bitbucketYML = `image: golang:1.25
pipelines:
  default:
    - step:
        name: lint
        image: golangci/golangci-lint:v2
        script:
          - golangci-lint run
    - parallel:
        - step:
            name: test
            script:
              - go test ./...
  branches:
    main:
      - step:
          name: deploy
          script:
            - ./deploy.sh
`

func scanFS(t *testing.T, files map[string]string) *model.Facts {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestScanBitbucket(t *testing.T) {
	facts := scanFS(t, map[string]string{"bitbucket-pipelines.yml": bitbucketYML})

	if p, _ := facts.Get("pipeline.provider"); p != "bitbucket" {
		t.Errorf("provider=%v want bitbucket", p)
	}
	steps, _ := facts.Get("pipeline.steps")
	if want := []string{"lint", "test", "deploy"}; !reflect.DeepEqual(steps, want) {
		t.Errorf("steps=%v want %v", steps, want)
	}
	images, _ := facts.Get("pipeline.images")
	if want := []string{"golang:1.25", "golangci/golangci-lint:v2"}; !reflect.DeepEqual(images, want) {
		t.Errorf("images=%v want %v", images, want)
	}
	// Position: the "lint" step name is on line 5.
	if loc := facts.ElemSource("pipeline.steps", 0); loc.File != "bitbucket-pipelines.yml" || loc.Line != 5 {
		t.Errorf("lint loc=%+v want bitbucket-pipelines.yml:5", loc)
	}
	if sc, _ := facts.Get("pipeline.step_count"); sc != 3 {
		t.Errorf("step_count=%v want 3", sc)
	}
}

const workflowYML = `name: ci
on: [push]
jobs:
  lint:
    runs-on: ubuntu-latest
    container: golangci/golangci-lint:v2
    steps:
      - uses: actions/checkout@v5
  test:
    runs-on: ubuntu-latest
    container:
      image: golang:1.25
    services:
      db:
        image: postgres:16
    steps:
      - run: go test ./...
`

func TestScanGitHub(t *testing.T) {
	facts := scanFS(t, map[string]string{".github/workflows/ci.yml": workflowYML})

	if p, _ := facts.Get("pipeline.provider"); p != "github" {
		t.Errorf("provider=%v want github", p)
	}
	steps, _ := facts.Get("pipeline.steps")
	if want := []string{"lint", "test"}; !reflect.DeepEqual(steps, want) {
		t.Errorf("steps=%v want %v", steps, want)
	}
	images, _ := facts.Get("pipeline.images")
	if want := []string{"golangci/golangci-lint:v2", "golang:1.25", "postgres:16"}; !reflect.DeepEqual(images, want) {
		t.Errorf("images=%v want %v", images, want)
	}
	// Position: job id "lint" is on line 4 of the workflow file.
	if loc := facts.ElemSource("pipeline.steps", 0); loc.File != ".github/workflows/ci.yml" || loc.Line != 4 {
		t.Errorf("lint loc=%+v want .github/workflows/ci.yml:4", loc)
	}
}

const gitlabYML = `image: golang:1.25
stages: [lint, test]
.hidden-template:
  script: [echo hi]
lint-job:
  stage: lint
  image: golangci/golangci-lint:v2
  script:
    - golangci-lint run
test-job:
  stage: test
  script:
    - go test ./...
`

func TestScanGitLab(t *testing.T) {
	facts := scanFS(t, map[string]string{".gitlab-ci.yml": gitlabYML})

	if p, _ := facts.Get("pipeline.provider"); p != "gitlab" {
		t.Errorf("provider=%v want gitlab", p)
	}
	steps, _ := facts.Get("pipeline.steps")
	if want := []string{"lint-job", "test-job"}; !reflect.DeepEqual(steps, want) {
		t.Errorf("steps=%v want %v (reserved + hidden keys must be excluded)", steps, want)
	}
	images, _ := facts.Get("pipeline.images")
	if want := []string{"golang:1.25", "golangci/golangci-lint:v2"}; !reflect.DeepEqual(images, want) {
		t.Errorf("images=%v want %v", images, want)
	}
	if loc := facts.ElemSource("pipeline.steps", 0); loc.Line != 5 {
		t.Errorf("lint-job loc=%+v want line 5", loc)
	}
}

func TestScanMultiProvider(t *testing.T) {
	facts := scanFS(t, map[string]string{
		"bitbucket-pipelines.yml":  bitbucketYML,
		".github/workflows/ci.yml": workflowYML,
	})
	if p, _ := facts.Get("pipeline.provider"); p != "bitbucket" {
		t.Errorf("provider=%v want bitbucket (detection order)", p)
	}
	providers, _ := facts.Get("pipeline.providers")
	if want := []string{"bitbucket", "github"}; !reflect.DeepEqual(providers, want) {
		t.Errorf("providers=%v want %v", providers, want)
	}
	steps, _ := facts.Get("pipeline.steps")
	if want := []string{"lint", "test", "deploy", "lint", "test"}; !reflect.DeepEqual(steps, want) {
		t.Errorf("steps=%v want %v", steps, want)
	}
	// images deduped across files: golang:1.25 appears in both, kept once.
	images, _ := facts.Get("pipeline.images")
	if want := []string{"golang:1.25", "golangci/golangci-lint:v2", "postgres:16"}; !reflect.DeepEqual(images, want) {
		t.Errorf("images=%v want %v", images, want)
	}
}

func TestScanNoPipeline(t *testing.T) {
	facts := scanFS(t, map[string]string{"go.mod": "module x\n"})
	if _, ok := facts.Get("pipeline.provider"); ok {
		t.Error("pipeline facts should be absent without pipeline files")
	}
}
