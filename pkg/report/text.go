package report

import (
	"bytes"
	"fmt"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

type textRenderer struct {
	color bool
}

func (t textRenderer) Render(r *Report) ([]byte, error) {
	s := newStyles(t.color)
	var b bytes.Buffer

	counts := map[model.Severity]int{}
	conformant := 0
	var digressing []string
	for _, res := range r.Results {
		if len(res.Findings) == 0 {
			conformant++
			fmt.Fprintf(&b, "%s %s\n", s.pass(), s.repo.Render(res.Repo.Slug()))
			continue
		}
		digressing = append(digressing, res.Repo.Slug())
		fmt.Fprintf(&b, "%s %s\n", s.fail(), s.repo.Render(res.Repo.Slug()))
		for _, f := range res.Findings {
			counts[f.Severity]++
			loc := ""
			if f.Location.File != "" {
				if f.Location.Line > 0 {
					loc = fmt.Sprintf("%s:%d", f.Location.File, f.Location.Line)
				} else {
					loc = f.Location.File
				}
				loc = "  " + s.loc.Render(loc)
			}
			fmt.Fprintf(&b, "    %s %s %s%s\n",
				s.badge(f.Severity),
				s.rule.Render("["+f.RuleID+"]"),
				f.Message,
				loc,
			)
			if f.Got != "" || f.Want != "" {
				fmt.Fprintf(&b, "        got %s, want %s\n",
					s.got.Render(quote(f.Got)),
					s.want.Render(quote(f.Want)),
				)
			}
		}
	}

	if len(digressing) > 0 {
		fmt.Fprintf(&b, "\n%s %s\n", s.summary.Render("digressing:"), join(digressing, ", "))
	}
	fmt.Fprintf(&b, "%s\n", s.summary.Render(summaryLine(len(r.Results), conformant, counts)))
	return b.Bytes(), nil
}

// summaryLine builds a one-line summary such as
// "12 scanned, 9 conformant, 3 digressing (3 error, 9 warn)".
func summaryLine(scanned, conformant int, counts map[model.Severity]int) string {
	digressing := scanned - conformant
	out := fmt.Sprintf("%d scanned, %d conformant, %d digressing", scanned, conformant, digressing)
	order := []model.Severity{model.SeverityError, model.SeverityDrift, model.SeverityWarn, model.SeverityInfo}
	var parts []string
	for _, sev := range order {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
	}
	if len(parts) > 0 {
		out += " (" + join(parts, ", ") + ")"
	}
	return out
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func quote(s string) string { return "\"" + s + "\"" }
