package report

import (
	"charm.land/lipgloss/v2"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// styles holds the lipgloss styles used by the text renderer
type styles struct {
	color   bool
	repo    lipgloss.Style
	rule    lipgloss.Style
	loc     lipgloss.Style
	got     lipgloss.Style
	want    lipgloss.Style
	ok      lipgloss.Style
	summary lipgloss.Style
	error   lipgloss.Style
	warn    lipgloss.Style
	drift   lipgloss.Style
	info    lipgloss.Style
}

// newStyles builds the style set. With color disabled all styles are plain.
func newStyles(color bool) styles {
	if !color {
		plain := lipgloss.NewStyle()
		return styles{
			color: false,
			repo:  plain, rule: plain, loc: plain, got: plain, want: plain,
			ok: plain, summary: plain, error: plain, warn: plain, drift: plain, info: plain,
		}
	}
	badge := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	return styles{
		color:   true,
		repo:    lipgloss.NewStyle().Bold(true).Underline(true),
		rule:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		loc:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true),
		got:     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		want:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		ok:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")),
		summary: lipgloss.NewStyle().Bold(true),
		error:   badge.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("1")),
		warn:    badge.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3")),
		drift:   badge.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("5")),
		info:    badge.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")),
	}
}

// badge renders a severity label
func (s styles) badge(sev model.Severity) string {
	if !s.color {
		return plainBadge(sev)
	}
	switch sev {
	case model.SeverityError:
		return s.error.Render("ERROR")
	case model.SeverityWarn:
		return s.warn.Render("WARN")
	case model.SeverityDrift:
		return s.drift.Render("DRIFT")
	case model.SeverityInfo:
		return s.info.Render("INFO")
	default:
		return string(sev)
	}
}

func plainBadge(sev model.Severity) string {
	switch sev {
	case model.SeverityError:
		return "ERROR"
	case model.SeverityWarn:
		return "WARN "
	case model.SeverityDrift:
		return "DRIFT"
	case model.SeverityInfo:
		return "INFO "
	default:
		return "?????"
	}
}

// pass renders the marker for a conformant repository.
func (s styles) pass() string {
	if !s.color {
		return "ok  "
	}
	return s.ok.Render("\u2714")
}

// fail renders the marker for a repository that digresses from the spec.
func (s styles) fail() string {
	if !s.color {
		return "FAIL"
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("\u2718")
}
