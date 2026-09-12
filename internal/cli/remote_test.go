package cli

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

func fakeRepos(n int) []model.Repo {
	repos := make([]model.Repo, n)
	for i := range repos {
		repos[i] = model.Repo{Provider: "github", Owner: "acme", Name: string(rune('a' + i))}
	}
	return repos
}

// TestScanRemoteReposBoundedParallelism proves the pool never exceeds the
// worker limit while still running repos concurrently.
func TestScanRemoteReposBoundedParallelism(t *testing.T) {
	const workers = 4
	var inFlight, maxSeen atomic.Int64
	worker := func(ctx context.Context, repo model.Repo) remoteResult {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			seen := maxSeen.Load()
			if cur <= seen || maxSeen.CompareAndSwap(seen, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond) // hold the slot so overlap is observable
		return remoteResult{repo: repo}
	}

	results := scanRemoteRepos(context.Background(), fakeRepos(20), workers, worker, nil)
	if len(results) != 20 {
		t.Fatalf("got %d results, want 20", len(results))
	}
	if m := maxSeen.Load(); m > workers {
		t.Errorf("max in-flight = %d, exceeds limit %d", m, workers)
	}
	if m := maxSeen.Load(); m < 2 {
		t.Errorf("max in-flight = %d, expected concurrent execution", m)
	}
}

// TestScanRemoteReposDeterministicOrder proves results keep the input order
// no matter which worker finishes first.
func TestScanRemoteReposDeterministicOrder(t *testing.T) {
	repos := fakeRepos(10)
	worker := func(ctx context.Context, repo model.Repo) remoteResult {
		if repo.Name == "a" { // make the first repo finish last
			time.Sleep(20 * time.Millisecond)
		}
		return remoteResult{repo: repo}
	}
	results := scanRemoteRepos(context.Background(), repos, 8, worker, nil)
	for i, res := range results {
		if res.repo.Name != repos[i].Name {
			t.Fatalf("results[%d] = %s, want %s (input order)", i, res.repo.Name, repos[i].Name)
		}
	}
}

// TestScanRemoteReposErrorIsSkip proves one failing repo becomes a skip with
// a reason instead of aborting the run.
func TestScanRemoteReposErrorIsSkip(t *testing.T) {
	repos := fakeRepos(3)
	worker := func(ctx context.Context, repo model.Repo) remoteResult {
		if repo.Name == "b" {
			return remoteResult{repo: repo, skip: "403 rate limited"}
		}
		return remoteResult{repo: repo, findings: []model.Finding{{RuleID: "r"}}}
	}

	var buf strings.Builder
	p := newProgress(&buf, false, false)
	p.begin(repos)
	results := scanRemoteRepos(context.Background(), repos, 2, worker, p)

	if results[1].skip != "403 rate limited" {
		t.Errorf("results[1].skip = %q, want the error reason", results[1].skip)
	}
	if len(results[0].findings) != 1 || len(results[2].findings) != 1 {
		t.Error("healthy repos must still produce findings")
	}
	if out := buf.String(); !strings.Contains(out, "(403 rate limited)") {
		t.Errorf("skip reason not reported in progress:\n%s", out)
	}
}
