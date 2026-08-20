# elostirion

[![PkgGoDev](https://pkg.go.dev/badge/github.com/whoisnjoguu/elostirion)](https://pkg.go.dev/github.com/whoisnjoguu/elostirion)

![elo catching drift across a demo fleet](demo/demo.gif)

`elostirion` applies desired-state management to fleets of repositories. You write a
single spec describing what every service must look like; base image, language
version, approved runtime images e.t.c and `elostirion` scans your repos against it
and verifies them in CI, so drift is caught before it merges. Convergence pull
requests and Terraform drift detection are next; see the [roadmap](#roadmap).

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
The spec can be a local path or a remote git URL pinned to a ref.

The common arguments are:

- The `--spec` flag to specify the spec, local or `git::` URL
- The `--format` flag to pick the output: text, json, junit, or sarif
- The `--fail-on` flag to set the minimum severity that fails the run

For example, verify the current repository in CI:

    $ elo verify --spec git::https://github.com/<your-org-name>/fleet-spec.git

Scan a directory of repositories (each immediate subdirectory is a repo) and
report which ones digress from the spec:

    $ elo scan --spec ./fleet-spec.yaml ~/src

See the CLI help (`-h` or `--help`) or below for full details.

[releases]: https://github.com/whoisnjoguu/elostirion/releases

### Commands

    verify   Check a single repository against the spec. Exits non-zero on
             violations. Intended as a CI step.

    scan     Read many repositories and report where they diverge from the
             spec, without making changes.

    apply    Evaluate repositories and print the change plan each violated
             rule's recipe would produce. The pull-request write path is in
             progress; see the roadmap.

    drift    Reserved for Terraform drift detection; not yet implemented.
             See the roadmap.

### Full

    Usage: elo <command> [options]

    Manage a fleet of repositories against a declarative spec.

    Elostirion reads a fleet spec describing the desired state of every
    repository; base image, language version, pipeline shape, Terraform
    module versions, required files, env contract .e.t.c and reports or fixes
    drift from that state. It does not require a server or a cluster.

    Common options:

      -s, --spec=path|url     The fleet spec. A local path or a 'git::' URL
                             with an optional '@ref' suffix. Required.

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

The near-term work, in rough order:

- **`apply` write path** — `apply` already computes per-repo change plans from
  recipes; opening the pull requests on GitHub and Bitbucket is in progress.
  Until then it reports what it would open (use `--dry-run`).
- **Terraform drift** — `drift` will three-way diff Terraform roots (HEAD,
  state, cloud) to fill the standalone drift-detection gap left by driftctl.
- **Remote scanning** — `--org` / `--repo` flags so `scan` can read every
  repository in a GitHub or Bitbucket org over the API (with `GITHUB_TOKEN` /
  `BITBUCKET_TOKEN`) instead of local checkouts.
- **More scanners** — Go, Python, and Dockerfile ship today; pipeline shape,
  required files, and env contracts are next.

## CI integration

`elostirion` is also built to run as a pipeline step, not just from a terminal.

Bitbucket Pipelines:

    - step:
        name: Fleet conformance
        image: golang:1.25
        script:
          - go install github.com/whoisnjoguu/elostirion/cmd/elo@latest
          - mkdir -p test-results
          - elo verify --spec git::https://bitbucket.org/<your-org-name>/fleet-spec.git@v1 --format junit > test-results/elo.xml

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

Alpha. The spec format, the CLI, and the library interface may all change.

## Contributing

Contributions are welcome. If reporting an issue with a scanner or a false-positive
drift result, please include the relevant repository files or Terraform configuration
if possible. A link to a public repository is most helpful.

## License

MIT
