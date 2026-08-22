package forge

import (
	"context"
	"fmt"

	"github.com/bluekeyes/patch2pr"
	"github.com/google/go-github/v90/github"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// ErrUnknownProvider is returned by For for an unrecognised provider.
func ErrUnknownProvider(provider string) error {
	return fmt.Errorf("forge: unknown provider %q", provider)
}

// defaultBranch derives the head branch name for a plan when none is set.
func defaultBranch(plan model.ChangePlan, opts Options) string {
	if opts.Branch != "" {
		return opts.Branch
	}
	recipe := plan.Recipe
	if recipe == "" {
		recipe = "converge"
	}
	return "elostirion/" + recipe
}

// githubForge opens pull requests through the GitHub Data API
type githubForge struct {
	client *github.Client
}

// newGitHubForge builds the backend
func newGitHubForge(cfg Config) (githubForge, error) {
	if cfg.Token == "" {
		return githubForge{}, fmt.Errorf("forge/github: no token; set GITHUB_TOKEN or pass --token")
	}
	opts := []github.ClientOptionsFunc{github.WithAuthToken(cfg.Token)}
	if cfg.BaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return githubForge{}, fmt.Errorf("forge/github: client: %w", err)
	}
	return githubForge{client: client}, nil
}

func (githubForge) Name() string { return "github" }

func (g githubForge) OpenPR(ctx context.Context, plan model.ChangePlan, opts Options) (Result, error) {
	branch := defaultBranch(plan, opts)
	if plan.Empty() {
		return Result{Branch: branch}, nil
	}
	if opts.DryRun {
		return Result{Branch: branch}, nil
	}
	owner, name := plan.Repo.Owner, plan.Repo.Name
	if owner == "" || name == "" {
		return Result{}, fmt.Errorf("forge/github: repo owner/name unknown for %s", plan.Repo.Slug())
	}
	repo := patch2pr.Repository{Owner: owner, Name: name}

	base := opts.BaseBranch
	if base == "" {
		r, _, err := g.client.Repositories.Get(ctx, owner, name)
		if err != nil {
			return Result{}, fmt.Errorf("forge/github: get repo: %w", err)
		}
		base = r.GetDefaultBranch()
	}
	baseRef, _, err := g.client.Git.GetRef(ctx, owner, name, "refs/heads/"+base)
	if err != nil {
		return Result{}, fmt.Errorf("forge/github: get base ref: %w", err)
	}
	baseSHA := baseRef.Object.GetSHA()

	// Build a tree of the full edited contents on top of the base tree.
	entries := make([]*github.TreeEntry, 0, len(plan.Edits))
	for _, e := range plan.Edits {
		if e.Delete {
			return Result{}, fmt.Errorf("forge/github: file deletion not supported yet (%s)", e.Path)
		}
		entries = append(entries, &github.TreeEntry{
			Path:    github.Ptr(e.Path),
			Mode:    github.Ptr("100644"),
			Type:    github.Ptr("blob"),
			Content: github.Ptr(string(e.Content)),
		})
	}
	tree, _, err := g.client.Git.CreateTree(ctx, owner, name, baseSHA, entries)
	if err != nil {
		return Result{}, fmt.Errorf("forge/github: create tree: %w", err)
	}

	msg := plan.Title
	if msg == "" {
		msg = "chore: converge to fleet spec"
	}
	commit, _, err := g.client.Git.CreateCommit(ctx, owner, name, github.Commit{
		Message: github.Ptr(msg),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: github.Ptr(baseSHA)}},
	}, nil)
	if err != nil {
		return Result{}, fmt.Errorf("forge/github: create commit: %w", err)
	}

	// patch2pr.Reference.Set handles the create-vs-update-with-404 logic.
	ref := patch2pr.NewReference(g.client, repo, "refs/heads/"+branch)
	if err := ref.Set(ctx, commit.GetSHA(), true); err != nil {
		return Result{}, fmt.Errorf("forge/github: set ref: %w", err)
	}

	// Reuse an open PR for this head branch when one exists.
	existing, _, err := g.client.PullRequests.List(ctx, owner, name, &github.PullRequestListOptions{
		State: "open",
		Head:  owner + ":" + branch,
	})
	if err == nil && len(existing) > 0 {
		return Result{URL: existing[0].GetHTMLURL(), Branch: branch, Created: false}, nil
	}

	pr, err := ref.PullRequest(ctx, github.CreatePullRequest{
		Title: github.Ptr(msg),
		Base:  base,
		Body:  github.Ptr(plan.Body),
		Draft: github.Ptr(opts.Draft),
	})
	if err != nil {
		return Result{}, fmt.Errorf("forge/github: create pr: %w", err)
	}
	return Result{URL: pr.GetHTMLURL(), Branch: branch, Created: true}, nil
}

// notImplemented is used for providers whose backend is not built yet.
type notImplemented struct{ provider string }

func (n notImplemented) Name() string { return n.provider }

func (n notImplemented) OpenPR(_ context.Context, _ model.ChangePlan, _ Options) (Result, error) {
	return Result{}, fmt.Errorf("forge/%s: backend not yet implemented", n.provider)
}
