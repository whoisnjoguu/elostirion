package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/v90/github"
	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/reconcile"
	"github.com/whoisnjoguu/elostirion/pkg/report"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
	"github.com/whoisnjoguu/elostirion/pkg/vcs"
)

var verifyRef string

// verifyCmd checks a single repository against the spec
var verifyCmd = &cobra.Command{
	Use:   "verify [dir | owner/name | url]",
	Short: "Check a single repository against the spec. Exits non-zero on violations.",
	Long: `Verify checks a single repository against the spec and exits non-zero when
there are violations at or above the --fail-on severity.

The target is a local checkout by default, so CI needs no credentials. It can
also be a remote GitHub repository given as owner/name or a github.com URL; the
tree is read through the GitHub API without cloning (set GITHUB_TOKEN for
private repos, use --ref for a branch, tag, or SHA).

With --format junit the results appear in Bitbucket's test report UI; with
--format sarif they annotate a GitHub pull request diff.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().StringVar(&verifyRef, "ref", "", "branch, tag, or SHA for remote targets (default: default branch)")
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	s, err := loadSpec()
	if err != nil {
		return err
	}

	if owner, name, ok := remoteTarget(target); ok {
		return verifyRemote(s, owner, name)
	}

	slug := filepath.Base(mustAbs(target))
	findings, repo, err := evaluate(s, target, slug)
	if err != nil {
		return err
	}
	rep := report.New(s, repo, findings)
	return renderAndExit(rep)
}

// slugRe matches a bare owner/name target.
var slugRe = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

// remoteTarget reports whether target names a remote GitHub repository
func remoteTarget(target string) (owner, name string, ok bool) {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "git@") {
		provider, o, n, err := vcs.ParseRemote(target)
		if err != nil || provider != "github" {
			return "", "", false
		}
		return o, n, true
	}
	if slugRe.MatchString(target) {
		if _, err := os.Stat(target); err == nil {
			return "", "", false // an existing local directory wins
		}
		parts := strings.SplitN(target, "/", 2)
		return parts[0], parts[1], true
	}
	return "", "", false
}

// verifyRemote scans a GitHub repository through the API, without cloning.
func verifyRemote(s *spec.Spec, owner, name string) error {
	ctx := context.Background()
	var clientOpts []github.ClientOptionsFunc
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		clientOpts = append(clientOpts, github.WithAuthToken(token))
	}
	client, err := github.NewClient(clientOpts...)
	if err != nil {
		return failure("github client: %v", err)
	}
	fsys, err := vcs.NewGitHubFS(ctx, client, owner, name, verifyRef)
	if err != nil {
		return failure("%v", err)
	}
	repo := model.Repo{Provider: "github", Owner: owner, Name: name, Ref: verifyRef}
	facts, err := scan.Run(fsys, repo, languages...)
	if err != nil {
		return failure("scan %s/%s: %v", owner, name, err)
	}
	findings := reconcile.Evaluate(s, facts)
	rep := report.New(s, facts.Repo, findings)
	return renderAndExit(rep)
}

// mustAbs returns the absolute path of dir, falling back to dir on error.
func mustAbs(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
