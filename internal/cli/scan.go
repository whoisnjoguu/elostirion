package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
	pkgreader "github.com/whoisnjoguu/elostirion/pkg/reader"
	"github.com/whoisnjoguu/elostirion/pkg/reconcile"
	"github.com/whoisnjoguu/elostirion/pkg/report"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
	"github.com/whoisnjoguu/elostirion/pkg/vcs"
)

// Remote scan targets.
var (
	remoteFlag []string
	orgFlag    string
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

	scanCmd.Flags().StringSliceVar(&remoteFlag, "remote", nil,
		"scan a remote repository directly without a clone, e.g. github.com/acme/api (repeatable)")
	scanCmd.Flags().StringVar(&orgFlag, "org", "",
		"scan every repository in an organisation, e.g. github.com/acme")
	scanCmd.MarkFlagsMutuallyExclusive("remote", "org")
}

func runScan(cmd *cobra.Command, args []string) error {
	s, err := loadSpec()
	if err != nil {
		return err
	}

	if len(remoteFlag) > 0 || orgFlag != "" {
		if len(args) > 0 {
			return failure("--remote and --org cannot be combined with directory arguments")
		}
		return runScanRemote(s)
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

// runScanRemote audits repositories read directly from a provider API.
func runScanRemote(s *spec.Spec) error {
	ctx := context.Background()
	repos, err := remoteTargets(ctx)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return failure("no repositories to scan")
	}

	rep := &report.Report{SpecName: s.Name}
	for _, repo := range repos {
		reader, err := pkgreader.For(repo, forge.Config{Token: resolveToken(repo.Provider)})
		if err != nil {
			return failure("%v", err)
		}
		// listing the root authenticates the reader and drives marker filtering
		entries, err := reader.ListFiles(ctx, "")
		if err != nil {
			return failure("%s: %v", repo.Slug(), err)
		}
		if len(languages) > 0 && !hasMarker(entries, languages) {
			continue // no relevant marker for the selected languages
		}
		facts, err := scan.Run(pkgreader.FS(ctx, reader), repo, languages...)
		if err != nil {
			return failure("scan %s: %v", repo.Slug(), err)
		}
		rep.Add(facts.Repo, reconcile.Evaluate(s, facts))
	}
	return renderAndExit(rep)
}

// remoteTargets resolves the --remote or --org flags into repositories to scan.
func remoteTargets(ctx context.Context) ([]model.Repo, error) {
	if orgFlag != "" {
		provider, org, err := parseOrg(orgFlag)
		if err != nil {
			return nil, failure("%v", err)
		}
		repos, err := pkgreader.ListOrgRepos(ctx, provider, org, forge.Config{Token: resolveToken(provider)})
		if err != nil {
			return nil, failure("list org %s: %v", org, err)
		}
		return repos, nil
	}
	repos := make([]model.Repo, 0, len(remoteFlag))
	for _, r := range remoteFlag {
		repo, err := parseRemoteRepo(r)
		if err != nil {
			return nil, failure("%v", err)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

// parseRemoteRepo parses a host/owner/name target into a Repo.
func parseRemoteRepo(s string) (model.Repo, error) {
	provider, owner, name, err := vcs.ParseRemote("https://" + stripScheme(s))
	if err != nil {
		return model.Repo{}, fmt.Errorf("invalid --remote %q: %w", s, err)
	}
	return model.Repo{Provider: provider, Owner: owner, Name: name}, nil
}

// parseOrg parses a host/org target into a provider and organisation name.
func parseOrg(s string) (provider, org string, err error) {
	parts := strings.Split(strings.Trim(stripScheme(s), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid --org %q; want host/org, e.g. github.com/acme", s)
	}
	switch parts[0] {
	case "github.com":
		provider = "github"
	case "bitbucket.org":
		provider = "bitbucket"
	default:
		provider = parts[0]
	}
	return provider, parts[1], nil
}

// stripScheme removes a leading http(s):// from a target.
func stripScheme(s string) string {
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	return s
}

// hasMarker reports whether any root entry is a discovery marker for langs.
func hasMarker(entries []pkgreader.DirEntry, langs []string) bool {
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			present[e.Name] = true
		}
	}
	for _, m := range scan.MarkersFor(langs...) {
		if present[m] {
			return true
		}
	}
	return false
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
