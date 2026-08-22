package spec

// DefaultTemplate is a starter fleet-spec.yaml written by `elo spec init`. It targets
// the kinds of drift a small Go microservice team most often cares about.
const DefaultTemplate = `version: 1
name: fleet

rules:
  - id: go-version-min
    description: Go version in go.mod must be at least 1.24.
    severity: error
    scanner: gomod
    field: go_version
    op: gte
    value: "1.24"
    recipe: bump-language-version

  - id: dockerfile-present
    description: Every service must ship a Dockerfile.
    severity: error
    scanner: dockerfile
    field: base_image
    op: exists

  - id: base-image-approved
    description: Dockerfile base image must be an approved golang tag.
    severity: warn
    scanner: dockerfile
    field: base_image
    op: matches
    value: '^golang:1\.(2[4-9]|[3-9][0-9])'
    recipe: bump-base-image
    target: "golang:1.25"
`

// Default returns the parsed default spec.
func Default() (*Spec, error) {
	return Parse([]byte(DefaultTemplate))
}
