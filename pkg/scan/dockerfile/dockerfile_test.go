package dockerfile

import (
	"testing"
	"testing/fstest"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func TestScanMultiStage(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte(`
FROM golang:1.25 AS builder
RUN go build -o app .

FROM alpine:latest
COPY --from=builder /app /app
`)},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	got, ok := facts.Get("dockerfile.base_image")
	if !ok {
		t.Fatal("base_image not set")
	}
	if got != "alpine:latest" {
		t.Errorf("base_image=%v want alpine:latest", got)
	}
	if sc, _ := facts.Get("dockerfile.stage_count"); sc != 2 {
		t.Errorf("stage_count=%v want 2", sc)
	}
	if bi, _ := facts.Get("dockerfile.builder_image"); bi != "golang:1.25" {
		t.Errorf("builder_image=%v want golang:1.25", bi)
	}
	// builder_image points at the builder stage's FROM line, not the final stage.
	if loc := facts.Sources["dockerfile.builder_image"]; loc.Line != 2 {
		t.Errorf("builder_image line=%d want 2", loc.Line)
	}
}

// A stage aliased "builder" is chosen even when it is not the first FROM.
func TestScanBuilderStageWins(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte(`
FROM debian:bookworm AS base
FROM golang:1.25 AS builder
FROM gcr.io/distroless/base
`)},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	if bi, _ := facts.Get("dockerfile.builder_image"); bi != "golang:1.25" {
		t.Errorf("builder_image=%v want golang:1.25", bi)
	}
	if got, _ := facts.Get("dockerfile.base_image"); got != "gcr.io/distroless/base" {
		t.Errorf("base_image=%v want gcr.io/distroless/base", got)
	}
}

// Without a builder alias, builder_image falls back to the first FROM.
func TestScanBuilderFallsBackToFirstFrom(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte(`
FROM golang:1.25
FROM alpine:latest
`)},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	if bi, _ := facts.Get("dockerfile.builder_image"); bi != "golang:1.25" {
		t.Errorf("builder_image=%v want golang:1.25", bi)
	}
}

// In a single-stage Dockerfile, builder_image equals base_image.
func TestScanSingleStage(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte("FROM alpine:latest\n")},
	}
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fsys, facts); err != nil {
		t.Fatal(err)
	}
	base, _ := facts.Get("dockerfile.base_image")
	builder, _ := facts.Get("dockerfile.builder_image")
	if base != "alpine:latest" || builder != base {
		t.Errorf("single-stage: base=%v builder=%v want both alpine:latest", base, builder)
	}
}

func TestScanNoDockerfile(t *testing.T) {
	facts := model.NewFacts(model.Repo{Name: "svc"})
	if err := (Scanner{}).Scan(fstest.MapFS{}, facts); err != nil {
		t.Fatal(err)
	}
	if _, ok := facts.Get("dockerfile.base_image"); ok {
		t.Error("base_image should be absent when no Dockerfile")
	}
}
