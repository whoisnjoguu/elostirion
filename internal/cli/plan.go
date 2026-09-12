package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/diff"
	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
	pkgreader "github.com/whoisnjoguu/elostirion/pkg/reader"
	"github.com/whoisnjoguu/elostirion/pkg/reconcile"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// planCmd shows the change plan apply would make, without opening pull requests
var planCmd = &cobra.Command{
	Use:   "plan [dir...]",
	Short: "Show the change plan apply would make. A dry-run of apply.",
	Long: `Plan evaluates repositories, runs the recipes of violated rules, and prints
		the unified diff of the edits apply would commit. It makes no changes.

		Targets are local checkouts by default. Use --remote to preview changes for a
		remote repository read through the provider API, or --org to preview every
		repository in an organisation, neither of which is cloned.`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
	addDepthFlag(planCmd)
	planCmd.Flags().StringSliceVar(&remoteFlag, "remote", nil,
		"preview a remote repository without a clone, e.g. github.com/acme/api (repeatable)")
	planCmd.Flags().StringVar(&orgFlag, "org", "",
		"preview every repository in an organisation, e.g. github.com/acme")
	planCmd.MarkFlagsMutuallyExclusive("remote", "org")
}

func runPlan(cmd *cobra.Command, args []string) error {
	s, err := loadSpec()
	if err != nil {
		return err
	}

	if len(remoteFlag) > 0 || orgFlag != "" {
		if len(args) > 0 {
			return failure("--remote and --org cannot be combined with directory arguments")
		}
		return runPlanRemote(s)
	}

	roots := args
	if len(roots) == 0 {
		roots = []string{"."}
	}
	dirs, err := collectRepos(roots)
	if err != nil {
		return failure("discover repos: %v", err)
	}

	changed := false
	for _, dir := range dirs {
		plan, err := buildPlan(s, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			continue
		}
		if renderPlan(plan, scan.DirFS(dir)) {
			changed = true
		}
	}
	if !changed {
		fmt.Fprintln(os.Stdout, "no changes planned")
	}
	return nil
}

// runPlanRemote previews plans for repositories read directly from a provider
// API, streaming progress as scan does
func runPlanRemote(s *spec.Spec) error {
	ctx := context.Background()
	var p *progress
	if !quiet {
		p = newProgress(os.Stderr, useColorErr(), stderrIsTTY())
	}
	repos, err := remoteTargets(ctx, p)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return failure("no repositories to plan")
	}
	p.begin(repos)

	var out []planned
	for i, repo := range repos {
		reader, err := pkgreader.For(repo, forge.Config{Token: resolveToken(repo.Provider)})
		if err != nil {
			return failure("%v", err)
		}
		entries, err := reader.ListFiles(ctx, "")
		if err != nil {
			return failure("%s: %v", repo.Slug(), err)
		}
		if len(languages) > 0 && !hasMarker(entries, languages) {
			p.skipping(i+1, repo.Slug(), "no "+strings.Join(languages, "/")+" markers")
			continue
		}
		p.scanning(i+1, repo.Slug())
		fsys := pkgreader.FS(ctx, reader)
		facts, err := scan.Run(fsys, repo, languages...)
		if err != nil {
			return failure("scan %s: %v", repo.Slug(), err)
		}
		findings := reconcile.EvaluateFS(s, facts, fsys)
		plan := buildPlanFS(s, fsys, facts.Repo, findings)
		if !plan.Empty() || len(plan.Reasons) > 0 {
			out = append(out, planned{plan: plan, fsys: fsys})
		}
	}
	p.done()

	changed := false
	for _, pl := range out {
		if renderPlan(pl.plan, pl.fsys) {
			changed = true
		}
	}
	if !changed {
		fmt.Fprintln(os.Stdout, "no changes planned")
	}
	return nil
}

// planned pairs a plan with the filesystem its diff is computed against.
type planned struct {
	plan model.ChangePlan
	fsys fs.FS
}

// renderPlan prints a plan's header, reasons, and unified diff
func renderPlan(plan model.ChangePlan, fsys fs.FS) bool {
	if plan.Empty() && len(plan.Reasons) == 0 {
		return false
	}
	fmt.Fprintf(os.Stdout, "%s (branch elostirion/%s)\n", plan.Repo.Slug(), planBranch(plan.Recipe))
	for _, r := range plan.Reasons {
		fmt.Fprintf(os.Stdout, "  %s\n", r)
	}
	if d := diff.ForPlan(fsys, plan); d != "" {
		if useColor() {
			d = diff.Colorize(d)
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprint(os.Stdout, d)
	}
	fmt.Fprintln(os.Stdout)
	return true
}

func planBranch(recipeName string) string {
	if recipeName == "" {
		return "converge"
	}
	return recipeName
}
