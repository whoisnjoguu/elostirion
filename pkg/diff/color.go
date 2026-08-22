// Package diff renders unified diffs and colorizes them for the terminal.
package diff

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	addStyle  = lipgloss.NewStyle().Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
	delStyle  = lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15"))
	hunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	fileStyle = lipgloss.NewStyle().Bold(true)
)

// Colorize highlights a unified diff for terminal display
func Colorize(diff string) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		switch {
		case line == "":
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			lines[i] = fileStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = hunkStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = addStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = delStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
