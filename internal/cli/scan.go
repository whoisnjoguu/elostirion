package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/report"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

// scanCmd reads many repositories and reports where they diverge from the spec without making changes
var scanCmd = &cobra.Command{
	Use:   "scan [dir...]",
	Short: "Read many repositories and report where they diverge from the spec.",
	Long: `Scan reads a number of repositories and reports where they diverge from the
spec without making changes. It is useful for auditing the state of a fleet.

Each argument is treated as a root directory whose immediate subdirectories are
the repositories to scan (a repository is a subdirectory carrying a discovery
marker for the selected languages, for example go.mod for --language go). With no
arguments the current directory is the root. If a root itself is a repository and
contains no sub-repositories, the root is scanned directly. Use --language to
restrict which scanners run.`,
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	s, err := loadSpec()
	if err != nil {
		return err
	}

	roots := args
	if len(roots) == 0 {
		roots = []string{"."}
	}

	dirs, err := collectRepos(roots)
	if err != nil {
		return failure("discover repos: %v", err)
	}
	if len(dirs) == 0 {
		return failure("no repositories found under %v", roots)
	}

	rep := &report.Report{SpecName: s.Name}
	for _, dir := range dirs {
		slug := filepath.Base(mustAbs(dir))
		findings, repo, err := evaluate(s, dir, slug)
		if err != nil {
			return err
		}
		rep.Add(repo, findings)
	}
	return renderAndExit(rep)
}

// collectRepos expands each root into the repositories to scan
func collectRepos(roots []string) ([]string, error) {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		key := mustAbs(dir)
		if !seen[key] {
			seen[key] = true
			dirs = append(dirs, dir)
		}
	}
	for _, root := range roots {
		found, err := discoverRepos(root)
		if err != nil {
			return nil, err
		}
		if len(found) == 0 && isRepo(root) {
			add(root)
			continue
		}
		for _, d := range found {
			add(d)
		}
	}
	return dirs, nil
}

// isRepo reports whether dir itself carries a discovery marker for the selected languages.
func isRepo(dir string) bool {
	for _, m := range scan.MarkersFor(languages...) {
		if fileExists(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}

// discoverRepos returns immediate subdirectories of root that contain a
// discovery marker for the selected languages, treating each as a repository to scan
func discoverRepos(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	markers := scan.MarkersFor(languages...)
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "." || e.Name() == ".git" {
			continue
		}
		dir := filepath.Join(root, e.Name())
		for _, m := range markers {
			if fileExists(filepath.Join(dir, m)) {
				dirs = append(dirs, dir)
				break
			}
		}
	}
	return dirs, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
