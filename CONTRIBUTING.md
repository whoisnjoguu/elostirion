# Contributing to elostirion

Thanks for your interest. Contributions of all kinds are welcome — bug reports, new scanners, spec-rule improvements, and documentation fixes.

## Before you start

- **Bug fixes and docs** — open a pull request directly.
- **New scanners, new CLI flags, or spec-format changes** — open an issue first so the design can be agreed before you write code. The project is in alpha and moving quickly; a short discussion saves rework.


## Pull requests

- Keep PRs focused — one logical change per PR.
- Run `gofmt`, `go vet ./...`, and `go test ./...` before pushing.
- Update the relevant section of `README.md` if you add a scanner or change CLI behaviour.
- The project is in alpha, so breaking changes to `pkg/model` or the spec format are acceptable but must be called out in the PR description.

## Reporting issues

When reporting a scanner bug or a false-positive drift result, include:

- The relevant file(s) the scanner reads (e.g. `go.mod`, `Dockerfile`), or a link to a public repository.
- The spec rule that triggered the finding.
- The actual vs. expected output (`elo scan --format json`).

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE) that covers the project.
