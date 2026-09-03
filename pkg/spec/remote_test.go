package spec

import "testing"

func TestParseGitSource(t *testing.T) {
	cases := []struct {
		name                       string
		source                     string
		owner, repo, path, refWant string
	}{
		{
			name:   "https with query ref",
			source: "git::https://github.com/acme/fleet//policies/fleet.yaml?ref=v1.2.3",
			owner:  "acme", repo: "fleet", path: "policies/fleet.yaml", refWant: "v1.2.3",
		},
		{
			name:   "https with at ref",
			source: "git::https://github.com/acme/fleet//fleet.yaml@main",
			owner:  "acme", repo: "fleet", path: "fleet.yaml", refWant: "main",
		},
		{
			name:   "dot git suffix, no ref",
			source: "git::https://github.com/acme/fleet.git//dir/fleet.yaml",
			owner:  "acme", repo: "fleet", path: "dir/fleet.yaml", refWant: "",
		},
		{
			name:   "scp form with query ref",
			source: "git::git@github.com:acme/fleet//fleet.yaml?ref=abc123",
			owner:  "acme", repo: "fleet", path: "fleet.yaml", refWant: "abc123",
		},
		{
			name:   "slash branch via query ref",
			source: "git::https://github.com/acme/fleet//fleet.yaml?ref=feature/x",
			owner:  "acme", repo: "fleet", path: "fleet.yaml", refWant: "feature/x",
		},
		{
			name:   "leading slash in path is cleaned",
			source: "git::https://github.com/acme/fleet///fleet.yaml",
			owner:  "acme", repo: "fleet", path: "fleet.yaml", refWant: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs, err := parseGitSource(c.source)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gs.provider != "github" {
				t.Errorf("provider = %q, want github", gs.provider)
			}
			if gs.owner != c.owner || gs.repo != c.repo {
				t.Errorf("owner/repo = %s/%s, want %s/%s", gs.owner, gs.repo, c.owner, c.repo)
			}
			if gs.path != c.path {
				t.Errorf("path = %q, want %q", gs.path, c.path)
			}
			if gs.ref != c.refWant {
				t.Errorf("ref = %q, want %q", gs.ref, c.refWant)
			}
		})
	}
}

func TestParseGitSourceErrors(t *testing.T) {
	cases := []string{
		"git::https://github.com/acme/fleet",          // no //path
		"git::https://github.com/acme/fleet//?ref=v1", // empty path
		"git::not a url//fleet.yaml",                  // unparsable url
	}
	for _, c := range cases {
		if _, err := parseGitSource(c); err == nil {
			t.Errorf("%q: expected error, got nil", c)
		}
	}
}

func TestLoadGitNonGitHubHost(t *testing.T) {
	_, err := loadGit("git::https://gitlab.com/acme/fleet//fleet.yaml")
	if err == nil {
		t.Fatal("expected unsupported-host error")
	}
}

func TestParseGitHubWebURL(t *testing.T) {
	cases := []struct {
		name                       string
		source                     string
		ok                         bool
		owner, repo, path, refWant string
	}{
		{
			name:   "blob url",
			source: "https://github.com/acme/fleet/blob/v1.2.0/policies/fleet.yaml",
			ok:     true, owner: "acme", repo: "fleet", path: "policies/fleet.yaml", refWant: "v1.2.0",
		},
		{
			name:   "raw path url",
			source: "https://github.com/acme/fleet/raw/main/fleet.yaml",
			ok:     true, owner: "acme", repo: "fleet", path: "fleet.yaml", refWant: "main",
		},
		{
			name:   "raw host url falls through to http",
			source: "https://raw.githubusercontent.com/acme/fleet/abc123/dir/fleet.yaml",
			ok:     false,
		},
		{
			name:   "plain non-github url",
			source: "https://example.com/specs/fleet.yaml",
			ok:     false,
		},
		{
			name:   "github repo root, not a file",
			source: "https://github.com/acme/fleet",
			ok:     false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs, ok := parseGitHubWebURL(c.source)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if !c.ok {
				return
			}
			if gs.owner != c.owner || gs.repo != c.repo || gs.path != c.path || gs.ref != c.refWant {
				t.Errorf("got %s/%s path=%q ref=%q; want %s/%s path=%q ref=%q",
					gs.owner, gs.repo, gs.path, gs.ref, c.owner, c.repo, c.path, c.refWant)
			}
		})
	}
}
