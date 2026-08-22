// Package forge turns a ChangePlan into a commit and pull request on a git provider.
package forge

import (
	"context"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// Result describes the outcome of opening a pull request.
type Result struct {
	URL     string
	Branch  string
	Created bool // false when PR already exists
}

// Options controls how a pull request is opened
type Options struct {
	Branch     string
	BaseBranch string
	Draft      bool
	DryRun     bool
	Labels     []string
}

// Config carries provider credentials and endpoint overrides.
type Config struct {
	Token   string // authenticates API calls
	BaseURL string // overrides the API endpoint for self-hosted installations
}

// Forge opens pull requests for a provider.
type Forge interface {
	Name() string                                                                    // provider identifier
	OpenPR(ctx context.Context, plan model.ChangePlan, opts Options) (Result, error) // applies plan to repo and opens (or updates) a PR
}

// For returns a forge backend for a provider
func For(provider string, cfg Config) (Forge, error) {
	switch provider {
	case "github":
		return newGitHubForge(cfg)
	case "bitbucket":
		return notImplemented{provider: "bitbucket"}, nil
	case "gitlab":
		return notImplemented{provider: "gitlab"}, nil
	default:
		return nil, ErrUnknownProvider(provider)
	}
}
