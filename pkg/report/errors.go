package report

import "fmt"

// errUnknownFormat returns the error used for an unsupported output format.
func errUnknownFormat(format string) error {
	return fmt.Errorf("report: unknown format %q (want text, json, junit, or sarif)", format)
}
