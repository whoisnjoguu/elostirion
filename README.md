# elostirion

[![PkgGoDev](https://pkg.go.dev/badge/github.com/whoisnjoguu/elostirion)](https://pkg.go.dev/github.com/whoisnjoguu/elostirion)

![elo catching drift across a demo fleet](demo/demo.gif)

`elostirion` applies desired-state management to fleets of repositories. You write a
single spec describing what every service must look like; base image, language
version, approved runtime images e.t.c and `elostirion` scans your repos against it,
verifies them in CI so drift is caught before it merges, and opens the pull
requests that converge digressing repos back to spec.

## Why?

Most tools that change many repositories at once are imperative: you write a script,
run it once, and forget it. There is no memory of what the fleet _should_ look like,
so the next drift goes unnoticed until something breaks in production.

`elostirion` is declarative instead. The spec is the source of truth, and reconciling
against it is a repeatable operation. Run it on a schedule to catch drift across every repo, or run it as a per-repo CI gate so a change can't merge unless it conforms.

As a library, the fleet model and scanners can be embedded in other tools: query
which services run an old language version, or which Dockerfiles diverge from the
golden template.

## Usage: CLI

Pre-built binaries for common platforms are available on the [releases][] page.

You can also install from source using `go install`:

    go install github.com/whoisnjoguu/elostirion/cmd/elo@latest

The CLI reads a fleet spec (`fleet-spec.yaml`) and operates on one or more repositories.
The spec is a local path today; remote `git::` specs are planned
([#8](https://github.com/whoisnjoguu/elostirion/issues/8)).

The common arguments are:

- The `--spec` flag to specify the spec file
- The `--format` flag to pick the output: text, json, junit, or sarif
- The `--fail-on` flag to set the minimum severity that fails the run

For example, verify the current repository in CI:

    $ elo verify --spec fleet-spec.yaml

Verify a remote GitHub repository through the API, without cloning (set
`GITHUB_TOKEN` for private repos, `--ref` for a branch, tag, or SHA):

    $ elo verify some-org/some-repo --spec fleet-spec.yaml

Scan a directory of repositories (each immediate subdirectory is a repo) and
report which ones digress from the spec:

    $ elo scan --spec ./fleet-spec.yaml ~/src

Show the diffs that would fix the digressing repos, then open the pull requests:

    $ elo plan --spec ./fleet-spec.yaml ~/src
    $ GITHUB_TOKEN=... elo apply --spec ./fleet-spec.yaml ~/src

See the CLI help (`-h` or `--help`) or below for full details.

[releases]: https://github.com/whoisnjoguu/elostirion/releases

### Commands

    verify   Check a single repository against the spec. Works on a local
             checkout or a remote GitHub repo (owner/name or URL) read through
             the API without cloning. Exits non-zero on violations. Intended
             as a CI step.

    scan     Read many repositories and report where they diverge from the
             spec, without making changes.

    plan     Run the recipes of violated rules and print the unified diff of
             the edits apply would commit. A dry-run of apply.

    apply    Open pull requests that converge repositories to the spec, using
             generic recipes (bump-language-version, bump-base-image). On
             GitHub the commit is created through the Git Data API; no clone.
             Bitbucket and GitLab backends are planned.

    spec     Author and check specs: init, validate, fmt.


### Full

    Usage: elo <command> [options]

    Manage a fleet of repositories against a declarative spec.

    Elostirion reads a fleet spec describing the desired state of every
    repository; base image, language version, pipeline shape, Terraform
    module versions, required files, env contract .e.t.c and reports or fixes
    drift from that state. It does not require a server or a cluster.

    Common options:

      -s, --spec=path         The fleet spec, a local file path. Required.
                             (Remote 'git::' specs are planned; see roadmap.)

      -f, --format=fmt        Output format: text, json, junit, or sarif.
                             junit surfaces results in Bitbucket's test report
                             UI; sarif annotates GitHub pull request diffs.
                             Default: text.

      --fail-on=severity      Minimum severity that causes a non-zero exit:
                             error, drift, or warn. Default: error.

      -l, --language=langs    Languages to scan (for example go, py);
                             repeatable or comma-separated. Default: all.

      -d, --dry-run           Report what would change without making changes.

      -v, --verbose           Enable verbose output.

    Exit codes:

      0  conformant
      1  violations or drift found
      2  tool error

## Roadmap

The near-term work, in rough order (tracked in the
[issues](https://github.com/whoisnjoguu/elostirion/issues)):

- **Bitbucket pull requests** — `apply` opens PRs on GitHub today; the
  Bitbucket backend is next
  ([#3](https://github.com/whoisnjoguu/elostirion/issues/3)).
- **Terraform drift** — `drift` will diff Terraform roots against state to
  fill the standalone drift-detection gap left by driftctl
  ([#7](https://github.com/whoisnjoguu/elostirion/issues/7)).
- **Org-wide remote scanning** — `elo scan --remote github.com/<org>` to sweep
  every repository in an org over the API, no checkouts
  ([#6](https://github.com/whoisnjoguu/elostirion/issues/6)).
- **Remote specs** — reference one central `fleet-spec.yaml` by `git::` URL
  pinned to a ref ([#8](https://github.com/whoisnjoguu/elostirion/issues/8)).
- **More scanners** — Go, Python, and Dockerfile ship today; node, pipeline
  shape, and env contracts are next
  ([#4](https://github.com/whoisnjoguu/elostirion/issues/4),
  [#5](https://github.com/whoisnjoguu/elostirion/issues/5)).

## CI integration

`elostirion` is also built to run as a pipeline step, not just from a terminal.

Bitbucket Pipelines:

    - step:
        name: Fleet conformance
        image: golang:1.25
        script:
          - go install github.com/whoisnjoguu/elostirion/cmd/elo@latest
          - mkdir -p test-results
          - elo verify --spec fleet-spec.yaml --format junit > test-results/elo.xml

GitHub Actions:

    - uses: actions/setup-go@v5
      with:
        go-version: "1.25"
    - run: go install github.com/whoisnjoguu/elostirion/cmd/elo@latest
    - run: elo verify --spec fleet-spec.yaml --format sarif > elo.sarif
    - uses: github/codeql-action/upload-sarif@v3
      if: always()
      with:
        sarif_file: elo.sarif

Complete pipeline examples live in [examples/ci](examples/ci).

In `verify` mode the binary only reads the checkout and the spec, so it needs no
credentials and runs fast. Rules carry a severity, so a fleet-wide change can be
rolled out as a warning before it becomes a merge-blocking error.

## Usage: Library

The CLI is built on the `elostirion` library, which can be used to build other tools
that model and reconcile repository fleets. See the [documentation][] for full
details.

    import "github.com/whoisnjoguu/elostirion/pkg/scan"

The library exposes the fleet model (`pkg/model`), the spec loader (`pkg/spec`),
the repository scanners (`pkg/scan`), and the reconciler (`pkg/reconcile`) that
evaluates a spec against scanned facts.

[documentation]: https://pkg.go.dev/github.com/whoisnjoguu/elostirion?tab=doc

## Stability

Alpha (v0.1.0). The spec format, the CLI, and the library interface may all
change.

## Contributing

Contributions are welcome. If reporting an issue with a scanner or a false-positive
drift result, please include the relevant repository files or Terraform configuration
if possible. A link to a public repository is most helpful.

## License

MIT
