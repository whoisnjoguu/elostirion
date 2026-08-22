package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/recipe"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
	"github.com/whoisnjoguu/elostirion/pkg/vcs"
)

var (
	applyProvider string
	applyToken    string
	applyDraft    bool
)

// applyCmd opens pull requests that converge repositories to the spec
var applyCmd = &cobra.Command{
	Use:   "apply [dir...]",
	Short: "Open pull requests that converge repositories to the spec.",
	Long: `Apply evaluates repositories against the spec, runs each violated rule's
recipe to produce real file edits, and opens a pull request per repository via
the forge backend. The repository owner and name are derived from the checkout's
origin remote. Use --dry-run to compute plans without writing anything.`,
	RunE: runApply,
}

func init() {
	applyCmd.Flags().StringVar(&applyProvider, "provider", "", "forge provider: github, bitbucket, gitlab (default: derived from origin remote)")
	applyCmd.Flags().StringVar(&applyToken, "token", "", "API token (default: GITHUB_TOKEN)")
	applyCmd.Flags().BoolVar(&applyDraft, "draft", false, "open pull requests as drafts")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
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

	ctx := context.Background()
	var opened, planned int
	for _, dir := range dirs {
		plan, err := buildPlan(s, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			continue
		}
		if plan.Empty() {
			continue
		}
		planned++
		fmt.Fprintf(os.Stdout, "%s: %d file edit(s) via %v\n", plan.Repo.Slug(), len(plan.Edits), plan.Reasons)

		provider := applyProvider
		if provider == "" {
			provider = plan.Repo.Provider
		}
		if provider == "" || provider == "local" {
			fmt.Fprintf(os.Stderr, "  skipped: no forge provider (no recognizable origin remote); pass --provider\n")
			continue
		}
		if dryRun {
			fmt.Fprintf(os.Stdout, "  dry-run: would push branch elostirion/%s and open PR via %s\n", planBranch(plan.Recipe), provider)
			continue
		}
		f, err := forge.For(provider, forge.Config{Token: resolveToken(provider)})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %v\n", err)
			continue
		}
		res, err := f.OpenPR(ctx, plan, forge.Options{Draft: applyDraft})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %v\n", err)
			continue
		}
		switch {
		case res.Created:
			opened++
			fmt.Fprintf(os.Stdout, "  opened %s\n", res.URL)
		case res.URL != "":
			fmt.Fprintf(os.Stdout, "  updated existing PR %s\n", res.URL)
		}
	}
	fmt.Fprintf(os.Stdout, "\n%d repo(s) with changes, %d PR(s) opened\n", planned, opened)
	return nil
}

// resolveToken picks the token flag or the provider's conventional env var
func resolveToken(provider string) string {
	if applyToken != "" {
		return applyToken
	}
	switch provider {
	case "github":
		return os.Getenv("GITHUB_TOKEN")
	case "bitbucket":
		return os.Getenv("BITBUCKET_TOKEN")
	case "gitlab":
		return os.Getenv("GITLAB_TOKEN")
	}
	return ""
}

// buildPlan evaluates one repo and runs the recipes of violated rules to
// produce a ChangePlan with real file edits
func buildPlan(s *spec.Spec, dir string) (model.ChangePlan, error) {
	slug := filepath.Base(mustAbs(dir))
	findings, repo, err := evaluate(s, dir, slug)
	if err != nil {
		return model.ChangePlan{}, err
	}
	if provider, owner, name, rerr := vcs.RemoteSlug(dir); rerr == nil {
		repo.Provider, repo.Owner, repo.Name = provider, owner, name
		repo.Path = dir
	}

	rulesByID := make(map[string]spec.Rule, len(s.Rules))
	for _, r := range s.Rules {
		rulesByID[r.ID] = r
	}

	fsys := scan.DirFS(dir)
	plan := model.ChangePlan{Repo: repo}
	edited := map[string][]byte{} // path -> latest content, so later recipes see earlier edits
	recipesRun := map[string]bool{}

	for _, f := range findings {
		rule, ok := rulesByID[f.RuleID]
		if !ok || rule.Recipe == "" {
			continue
		}
		r, ok := recipe.Get(rule.Recipe)
		if !ok {
			plan.Reasons = append(plan.Reasons, fmt.Sprintf("%s: unknown recipe %q", rule.ID, rule.Recipe))
			continue
		}
		edits, err := r.Apply(overlayFS{base: fsys, edits: edited}, rule, f)
		if err != nil {
			plan.Reasons = append(plan.Reasons, fmt.Sprintf("%s: %v", rule.ID, err))
			continue
		}
		for _, e := range edits {
			edited[e.Path] = e.Content
		}
		if len(edits) > 0 {
			recipesRun[rule.Recipe] = true
			plan.Reasons = append(plan.Reasons, fmt.Sprintf("%s via %s", rule.ID, rule.Recipe))
		}
	}

	for path, content := range edited {
		plan.Edits = append(plan.Edits, model.FileEdit{Path: path, Content: content})
	}
	if len(recipesRun) == 1 {
		for name := range recipesRun {
			plan.Recipe = name
		}
	} else if len(recipesRun) > 1 {
		plan.Recipe = "converge"
	}
	plan.Title = fmt.Sprintf("chore: converge %s to fleet spec", repo.Name)
	plan.Body = planBody(plan)
	return plan, nil
}

// planBody renders the PR body from the plan's reasons.
func planBody(plan model.ChangePlan) string {
	body := "Automated convergence by [elostirion](https://github.com/whoisnjoguu/elostirion).\n\n"
	for _, r := range plan.Reasons {
		body += "- " + r + "\n"
	}
	return body
}
