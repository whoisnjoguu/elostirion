package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// progress reports remote-scan progress on stderr, leaving stdout a clean
// report stream for --format json pipelines
type progress struct {
	w     io.Writer
	tty   bool
	start time.Time

	mu       sync.Mutex
	phase    string // "discovering" or "scanning"
	org      string
	slugNow  string
	i        int // current repo index (1-based)
	total    int
	width    int // slug column width for alignment
	cw       int // counter digit width
	scanned  int
	skipped  int
	frame    int
	animOn   bool
	stopAnim chan struct{}
	animDone chan struct{}

	scanVerb lipgloss.Style
	skipVerb lipgloss.Style
	slug     lipgloss.Style
	counter  lipgloss.Style
	reason   lipgloss.Style
	summary  lipgloss.Style
	spinner  lipgloss.Style
	barOn    lipgloss.Style
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	barWidth   = 20
	eraseLine  = "\r\x1b[2K"
	frameEvery = 100 * time.Millisecond
)

// newProgress builds a progress reporter writing to w
func newProgress(w io.Writer, color, tty bool) *progress {
	p := &progress{w: w, tty: tty, start: time.Now()}
	plain := lipgloss.NewStyle()
	p.scanVerb, p.skipVerb, p.slug, p.counter, p.reason, p.summary, p.spinner, p.barOn =
		plain, plain, plain, plain, plain, plain, plain, plain
	if color {
		p.scanVerb = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		p.skipVerb = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		p.slug = lipgloss.NewStyle().Bold(true)
		p.counter = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		p.reason = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
		p.summary = lipgloss.NewStyle().Bold(true)
		p.spinner = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
		p.barOn = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	}
	return p
}

// ensureAnim starts the spinner ticker once, TTY only.
func (p *progress) ensureAnim() {
	if !p.tty || p.animOn {
		return
	}
	p.animOn = true
	p.stopAnim = make(chan struct{})
	p.animDone = make(chan struct{})
	go func() {
		t := time.NewTicker(frameEvery)
		defer t.Stop()
		defer close(p.animDone)
		for {
			select {
			case <-p.stopAnim:
				return
			case <-t.C:
				p.mu.Lock()
				p.frame++
				p.redrawLocked()
				p.mu.Unlock()
			}
		}
	}()
}

// haltAnim stops the ticker and erases the animated line.
func (p *progress) haltAnim() {
	if !p.animOn {
		return
	}
	p.animOn = false
	close(p.stopAnim)
	<-p.animDone
	p.mu.Lock()
	fmt.Fprint(p.w, eraseLine)
	p.mu.Unlock()
}

// redrawLocked repaints the animated status line. Callers hold p.mu.
func (p *progress) redrawLocked() {
	spin := p.spinner.Render(spinnerFrames[p.frame%len(spinnerFrames)])
	switch p.phase {
	case "discovering":
		fmt.Fprintf(p.w, "%s%s discovering repositories in %s…", eraseLine, spin, p.org)
	case "scanning":
		padded := fmt.Sprintf("%-*s", p.width, p.slugNow)
		counter := p.counter.Render(fmt.Sprintf("%*d/%d", p.cw, p.i, p.total))
		fmt.Fprintf(p.w, "%s%s %s %s %s %s", eraseLine, spin,
			p.scanVerb.Render("scanning"), p.slug.Render(padded), p.barLocked(), counter)
	}
}

// barLocked renders the filled/empty progress blocks for i of total.
func (p *progress) barLocked() string {
	if p.total == 0 {
		return ""
	}
	filled := barWidth * p.i / p.total
	return p.barOn.Render(strings.Repeat("█", filled)) + strings.Repeat("░", barWidth-filled)
}

// discovering shows the org-enumeration phase
func (p *progress) discovering(org string) {
	if p == nil {
		return
	}
	if p.tty {
		p.mu.Lock()
		p.phase, p.org = "discovering", org
		p.redrawLocked()
		p.mu.Unlock()
		p.ensureAnim()
		return
	}
	fmt.Fprintf(p.w, "discovering repositories in %s...", org)
}

// discovered completes the discovery phase with the repo count.
func (p *progress) discovered(n int) {
	if p == nil {
		return
	}
	if p.tty {
		p.haltAnim()
		fmt.Fprintf(p.w, "discovering repositories in %s... %s\n", p.org, p.summary.Render(fmt.Sprintf("%d found", n)))
		return
	}
	fmt.Fprintf(p.w, " %s\n", p.summary.Render(fmt.Sprintf("%d found", n)))
}

// endLine terminates progress output before an error surfaces.
func (p *progress) endLine() {
	if p == nil {
		return
	}
	if p.tty {
		p.haltAnim()
		return
	}
	fmt.Fprintln(p.w)
}

// begin sizes the progress columns for the repo set and starts the bar.
func (p *progress) begin(repos []model.Repo) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.total = len(repos)
	p.cw = len(strconv.Itoa(p.total))
	for _, r := range repos {
		if n := len(r.Slug()); n > p.width {
			p.width = n
		}
	}
	p.phase = "scanning"
	p.mu.Unlock()
	p.ensureAnim()
}

// scanning reports repo i of total as being scanned.
func (p *progress) scanning(i int, slug string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.scanned++
	p.i, p.slugNow = i, slug
	if p.tty {
		p.redrawLocked()
	} else {
		p.lineLocked(p.scanVerb, "scanning", i, slug, "")
	}
	p.mu.Unlock()
}

// skipping reports repo i of total as skipped, with the reason
func (p *progress) skipping(i int, slug, reason string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.skipped++
	p.i, p.slugNow = i, slug
	if p.tty {
		fmt.Fprint(p.w, eraseLine)
		p.lineLocked(p.skipVerb, "skipping", i, slug, reason)
		p.redrawLocked()
	} else {
		p.lineLocked(p.skipVerb, "skipping", i, slug, reason)
	}
	p.mu.Unlock()
}

// lineLocked prints one aligned plain progress row
func (p *progress) lineLocked(verb lipgloss.Style, verbText string, i int, slug, reason string) {
	padded := fmt.Sprintf("%-*s", p.width, slug)
	counter := fmt.Sprintf("[%*d/%d]", p.cw, i, p.total)
	row := fmt.Sprintf("%s %s  %s", verb.Render(verbText), p.slug.Render(padded), p.counter.Render(counter))
	if reason != "" {
		row += " " + p.reason.Render("("+reason+")")
	}
	fmt.Fprintln(p.w, row)
}

// done stops any animation and prints the closing summary.
func (p *progress) done() {
	if p == nil {
		return
	}
	p.haltAnim()
	elapsed := time.Since(p.start).Round(time.Second)
	fmt.Fprintf(p.w, "%s\n\n", p.summary.Render(
		fmt.Sprintf("scanned %d repo(s), skipped %d, in %s", p.scanned, p.skipped, elapsed),
	))
}
