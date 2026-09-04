package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/reconcile"
	"github.com/whoisnjoguu/elostirion/pkg/report"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// exitError carries a specific process exit code out of a command's RunE.
type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return e.msg }

// failure returns a code-2 tool error.
func failure(format string, args ...any) exitError {
	return exitError{code: 2, msg: fmt.Sprintf(format, args...)}
}

// findingsExit returns a code-1 error signalling non-conformance with no extra message
func findingsExit() exitError { return exitError{code: 1} }

// validateLanguages rejects any --language value that no registered scanner knows
func validateLanguages(cmd *cobra.Command, args []string) error {
	if len(languages) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, l := range scan.Languages() {
		known[l] = true
	}
	for _, l := range languages {
		if !known[l] {
			return failure("unknown language %q; known languages: %v", l, scan.Languages())
		}
	}
	return nil
}

// loadSpec resolves the --spec flag into a parsed spec.
func loadSpec() (*spec.Spec, error) {
	if specFlag == "" {
		return nil, failure("no spec provided; pass --spec <path|git:: URL|http(s) URL>")
	}
	s, err := spec.Load(specFlag)
	if err != nil {
		return nil, failure("%v", err)
	}
	return s, nil
}

// thresholdSeverity maps the --fail-on flag to a severity.
func thresholdSeverity() model.Severity {
	switch failOn {
	case "warn":
		return model.SeverityWarn
	case "drift":
		return model.SeverityDrift
	case "error":
		return model.SeverityError
	default:
		return model.SeverityError
	}
}

// scanDir scans a local directory into Facts, tagging the repo as local
func scanDir(dir, slug string) (*model.Facts, error) {
	repo := model.Repo{Provider: "local", Name: slug, Path: dir}
	facts, err := scan.Run(scan.DirFS(dir), repo, languages...)
	if err != nil {
		return nil, failure("scan %s: %v", dir, err)
	}
	return facts, nil
}

// renderAndExit renders the report in the requested format, writes it to stdout,
// and returns the appropriate exit error based on --fail-on.
func renderAndExit(rep *report.Report) error {
	r, err := report.For(formatFlag, useColor())
	if err != nil {
		return failure("%v", err)
	}
	out, err := r.Render(rep)
	if err != nil {
		return failure("%v", err)
	}
	_, _ = os.Stdout.Write(out)
	if rep.ExceedsThreshold(thresholdSeverity()) {
		return findingsExit()
	}
	return nil
}

// useColor reports whether stdout should be colorized, honoring TTY detection,
// TERM, NO_COLOR, and CLICOLOR_FORCE via the color profile.
func useColor() bool {
	return colorprofile.Detect(os.Stdout, os.Environ()) >= colorprofile.ANSI
}

// evaluate scans a directory and evaluates the spec, returning findings.
func evaluate(s *spec.Spec, dir, slug string) ([]model.Finding, model.Repo, error) {
	facts, err := scanDir(dir, slug)
	if err != nil {
		return nil, model.Repo{}, err
	}
	return reconcile.Evaluate(s, facts), facts.Repo, nil
}
