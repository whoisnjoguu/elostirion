package cli

import (
	"context"
	"io/fs"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/forge"
	"github.com/whoisnjoguu/elostirion/pkg/model"
	pkgreader "github.com/whoisnjoguu/elostirion/pkg/reader"
	"github.com/whoisnjoguu/elostirion/pkg/reconcile"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// defaultConcurrency bounds parallel remote repo scans: enough to hide API
// latency without tripping provider secondary rate limits.
const defaultConcurrency = 8

var concurrencyFlag = defaultConcurrency

// addConcurrencyFlag registers --concurrency on a remote-capable command.
func addConcurrencyFlag(cmd *cobra.Command) {
	cmd.Flags().IntVar(&concurrencyFlag, "concurrency", defaultConcurrency,
		"max repositories scanned in parallel for --remote/--org")
}

// remoteResult is the outcome of scanning one remote repository. Exactly one
// of skip or facts is meaningful: a non-empty skip carries the reason the repo
// produced no findings (marker filter or per-repo error).
type remoteResult struct {
	repo     model.Repo
	fsys     fs.FS
	findings []model.Finding
	skip     string
}

// remoteWorker builds the per-repo scan function shared by scan and plan: it
// authenticates a reader, applies marker filtering, runs the scanners, and
// evaluates the spec. Errors become skips so one bad repo cannot abort an
// org-wide run.
func remoteWorker(s *spec.Spec) func(ctx context.Context, repo model.Repo) remoteResult {
	return func(ctx context.Context, repo model.Repo) remoteResult {
		reader, err := pkgreader.For(repo, forge.Config{Token: resolveToken(repo.Provider)})
		if err != nil {
			return remoteResult{repo: repo, skip: err.Error()}
		}
		// listing the root authenticates the reader and drives marker filtering
		entries, err := reader.ListFiles(ctx, "")
		if err != nil {
			return remoteResult{repo: repo, skip: err.Error()}
		}
		if len(languages) > 0 && !hasMarker(entries, languages) {
			return remoteResult{repo: repo, skip: "no " + strings.Join(languages, "/") + " markers"}
		}
		fsys := pkgreader.FS(ctx, reader)
		facts, err := scan.Run(fsys, repo, languages...)
		if err != nil {
			return remoteResult{repo: repo, skip: err.Error()}
		}
		return remoteResult{repo: facts.Repo, fsys: fsys, findings: reconcile.EvaluateFS(s, facts, fsys)}
	}
}

// scanRemoteRepos runs worker over repos with bounded parallelism, reporting
// per-repo progress as workers finish. Results keep the repos order so report
// output stays deterministic regardless of completion order.
func scanRemoteRepos(ctx context.Context, repos []model.Repo, workers int,
	worker func(context.Context, model.Repo) remoteResult, p *progress) []remoteResult {
	if workers < 1 {
		workers = 1
	}
	results := make([]remoteResult, len(repos))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := worker(ctx, repo)
			results[i] = res
			if res.skip != "" {
				p.skipping(res.repo.Slug(), res.skip)
			} else {
				p.scanning(res.repo.Slug())
			}
		}()
	}
	wg.Wait()
	return results
}
