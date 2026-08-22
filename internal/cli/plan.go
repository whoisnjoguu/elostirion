package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/diff"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

// planCmd shows the change plan apply would make, without opening pull requests
var planCmd = &cobra.Command{
	Use:   "plan [dir...]",
	Short: "Show the change plan apply would make. A dry-run of apply.",
	Long: `Plan evaluates repositories, runs the recipes of violated rules, and prints
the unified diff of the edits apply would commit. It makes no changes.`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
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

	changed := false
	for _, dir := range dirs {
		plan, err := buildPlan(s, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			continue
		}
		if plan.Empty() && len(plan.Reasons) == 0 {
			continue
		}
		changed = true
		fmt.Fprintf(os.Stdout, "%s (branch elostirion/%s)\n", plan.Repo.Slug(), planBranch(plan.Recipe))
		for _, r := range plan.Reasons {
			fmt.Fprintf(os.Stdout, "  %s\n", r)
		}
		if d := diff.ForPlan(scan.DirFS(dir), plan); d != "" {
			if useColor() {
				d = diff.Colorize(d)
			}
			fmt.Fprintln(os.Stdout)
			fmt.Fprint(os.Stdout, d)
		}
		fmt.Fprintln(os.Stdout)
	}
	if !changed {
		fmt.Fprintln(os.Stdout, "no changes planned")
	}
	return nil
}

func planBranch(recipeName string) string {
	if recipeName == "" {
		return "converge"
	}
	return recipeName
}
