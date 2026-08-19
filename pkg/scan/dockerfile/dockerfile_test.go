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
