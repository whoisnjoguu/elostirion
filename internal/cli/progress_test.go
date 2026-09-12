package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestProgressPlainOutput(t *testing.T) {
	var b strings.Builder
	p := newProgress(&b, false, false)

	p.discovering("acme")
	p.discovered(3)
	p.begin([]model.Repo{
		{Owner: "acme", Name: "api"},
		{Owner: "acme", Name: "worker"},
		{Owner: "acme", Name: "design-assets"},
	})
	p.scanning(1, "acme/api")
	p.scanning(2, "acme/worker")
	p.skipping(3, "acme/design-assets", "no go markers")
	p.done()

	out := b.String()
	if ansiRe.MatchString(out) {
		t.Errorf("plain progress contains ANSI sequences:\n%q", out)
	}
	for _, want := range []string{
		"discovering repositories in acme... 3 found",
		"scanning acme/api",
		"[1/3]",
		"skipping acme/design-assets  [3/3] (no go markers)",
		"scanned 2 repo(s), skipped 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("progress output missing %q:\n%s", want, out)
		}
	}
	// Slug column is padded so counters align across rows.
	lines := strings.Split(out, "\n")
	var cols []int
	for _, l := range lines {
		if i := strings.Index(l, "["); i >= 0 && strings.Contains(l, "/3]") {
			cols = append(cols, i)
		}
	}
	for i := 1; i < len(cols); i++ {
		if cols[i] != cols[0] {
			t.Errorf("counter columns not aligned: %v\n%s", cols, out)
		}
	}
}

func TestProgressColorOutput(t *testing.T) {
	var b strings.Builder
	p := newProgress(&b, true, false)
	p.begin([]model.Repo{{Owner: "acme", Name: "api"}})
	p.scanning(1, "acme/api")
	if !ansiRe.MatchString(b.String()) {
		t.Error("color progress has no ANSI sequences")
	}
}

func TestProgressTTYAnimates(t *testing.T) {
	var b strings.Builder
	p := newProgress(&b, false, true)

	p.discovering("acme")
	p.discovered(2)
	p.begin([]model.Repo{{Owner: "acme", Name: "api"}, {Owner: "acme", Name: "assets"}})
	p.scanning(1, "acme/api")
	p.skipping(2, "acme/assets", "no go markers")
	p.done()

	out := b.String()
	if !strings.Contains(out, "\r\x1b[2K") {
		t.Error("TTY mode should redraw in place with erase-line sequences")
	}
	if !strings.Contains(out, "█") || !strings.Contains(out, "░") {
		t.Errorf("TTY mode should render a progress bar:\n%q", out)
	}
	for _, spin := range spinnerFrames {
		if strings.Contains(out, spin) {
			return // at least one spinner frame drawn
		}
	}
	t.Errorf("TTY mode should render a spinner frame:\n%q", out)
}

func TestProgressTTYPersistsSkipsAndSummary(t *testing.T) {
	var b strings.Builder
	p := newProgress(&b, false, true)
	p.begin([]model.Repo{{Owner: "acme", Name: "api"}, {Owner: "acme", Name: "assets"}})
	p.scanning(1, "acme/api")
	p.skipping(2, "acme/assets", "no go markers")
	p.done()

	out := b.String()
	if !strings.Contains(out, "skipping acme/assets") || !strings.Contains(out, "(no go markers)") {
		t.Errorf("skip line must persist in TTY mode:\n%q", out)
	}
	if !strings.Contains(out, "scanned 1 repo(s), skipped 1") {
		t.Errorf("summary missing:\n%q", out)
	}
	// The animated line is erased before the summary prints.
	if !strings.Contains(strings.TrimRight(out, "\n"), "scanned 1 repo(s), skipped 1, in ") {
		t.Errorf("summary should close the stream:\n%q", out)
	}
}

func TestProgressNilIsQuiet(t *testing.T) {
	var p *progress // --quiet: nil reporter, every method a no-op
	p.discovering("acme")
	p.discovered(1)
	p.begin(nil)
	p.scanning(1, "x")
	p.skipping(2, "y", "z")
	p.endLine()
	p.done()
}
