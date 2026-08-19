# elostirion

[![PkgGoDev](https://pkg.go.dev/badge/github.com/whoisnjoguu/elostirion)](https://pkg.go.dev/github.com/whoisnjoguu/elostirion)


`elostirion` applies desired-state management to fleets of repositories. You write a
single spec describing what every service must look like; base image, language
version, pipeline shape, Terraform module versions, required files, env contract e.t.c
and `elostirion` scans your repos against it, verifies them in CI, and opens pull
requests that converge the ones that have drifted.

## Why?

Most tools that change many repositories at once are imperative: you write a script,
run it once, and forget it. There is no memory of what the fleet _should_ look like,
so the next drift goes unnoticed until something breaks in production.

`elostirion` is declarative instead. The spec is the source of truth, and reconciling
against it is a repeatable operation. Run it on a schedule to catch drift across every repo and Terraform root, or run it as a per-repo CI gate so a change can't merge unless it conforms.

As a library, the fleet model and scanners can be embedded in other tools: query
which services run an old language version, which Dockerfiles diverge from the golden
template, or which Terraform roots have drifted from their state.

## Usage: CLI

Pre-built binaries for common platforms are available on the [releases][] page.

You can also install from source using `go install`:

    go install github.com/whoisnjoguu/elostirion/cmd/elo@latest

The CLI reads a fleet spec (`fleet-spec.yaml`) and operates on one or more repositories.
The spec can be a local path or a remote git URL pinned to a ref.

The common arguments are:

- The `-spec` flag to specify the spec, local or `git::` URL
- A token, set with the `-token` flag or in the `GITHUB_TOKEN` / `BITBUCKET_TOKEN`
  environment variable, for modes that read remote repos or open pull requests

For example, verify the current repository in CI:

    $ elo verify -spec git::https://github.com/<your-org-name>/fleet-spec.git

Scan an entire org and open convergence pull requests:

    $ export GITHUB_TOKEN="token"
    $ elo scan -spec ./fleet-spec.yaml -org <your-org-name>
    $ elo apply -spec ./fleet-spec.yaml -org <your-org-name> --recipe bump-go

See the CLI help (`-h` or `-help`) or below for full details.

[releases]: https://github.com/whoisnjoguu/elostirion/releases

### Commands

    verify   Check a single repository against the spec. Exits non-zero on
             violations. Intended as a CI step.

    scan     Read many repositories and report where they diverge from the
             spec, without making changes.

    drift    Three-way diff Terraform roots (HEAD, state, cloud) and report
             drift. Fills the standalone drift-detection gap left by driftctl.

    apply    Open pull requests that converge repositories to the spec, using
             named recipes.

### Full

    Usage: elo <command> [options]

    Manage a fleet of repositories against a declarative spec.

    Elostirion reads a fleet spec describing the desired state of every
    repository; base image, language version, pipeline shape, Terraform
    module versions, required files, env contract .e.t.c and reports or fixes
    drift from that state. It does not require a server or a cluster.

    Common options:

      -spec=path|url         The fleet spec. A local path or a 'git::' URL
                             with an optional '@ref' suffix. Required.

      -format=fmt            Output format: text, json, junit, or sarif.
                             junit surfaces results in Bitbucket's test report
                             UI; sarif annotates GitHub pull request diffs.
                             Default: text.

      -fail-on=severity      Minimum severity that causes a non-zero exit:
                             error, warn, or drift. Default: error.

      -token=token           API token for the target platform. If unset, use
                             GITHUB_TOKEN or BITBUCKET_TOKEN.

      -org=org               Target every repository in the given org.

      -repo=owner/name       Target a single repository. Can be repeated.

      -tf-roots=glob         Glob of Terraform root directories for 'drift'.

      -recipe=name           Named convergence recipe for 'apply'.

      -dry-run               Report what would change without opening pull
                             requests.

      -v/-version            Print the version and exit.

    Exit codes:

      0  conformant
      1  violations or drift found
      2  tool error

## CI integration

`elostirion` is also built to run as a pipeline step, not just from a terminal.

Bitbucket Pipelines:

    - step:
        name: Fleet conformance
        image: whoisnjoguu/elostirion:1
        script:
          - elo verify -spec git::https://bitbucket.org/<your-org-name>/fleet-spec.git@v1 -format junit > test-results/elo.xml

GitHub Actions:

    - uses: whoisnjoguu/elostirion-action@v1
      with:
        spec: <your-org-name>/fleet-spec
        fail-on: error

In `verify` mode the binary only reads the checkout and the spec, so it needs no
credentials and runs fast. Rules carry a severity, so a fleet-wide change can be
rolled out as a warning before it becomes a merge-blocking error.

## Usage: Library

The CLI is built on the `elostirion` library, which can be used to build other tools
that model and reconcile repository fleets. See the [documentation][] for full
details.

    import "github.com/whoisnjoguu/elostirion"

The library exposes the fleet model, the repository and Terraform scanners, and the
reconciler that turns a diff into a change plan.

[documentation]: https://pkg.go.dev/github.com/whoisnjoguu/elostirion?tab=doc

## Stability

Alpha. The spec format, the CLI, and the library interface may all change.

## Contributing

Contributions are welcome. If reporting an issue with a scanner or a false-positive
drift result, please include the relevant repository files or Terraform configuration
if possible. A link to a public repository is most helpful.

## License

MIT
