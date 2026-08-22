package recipe

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

func init() {
	Register(bumpLanguageVersion{})
	Register(bumpBaseImage{})
}

// bumpLanguageVersion is the generic language-version recipe
type bumpLanguageVersion struct{}

func (bumpLanguageVersion) Name() string { return "bump-language-version" }

func (bumpLanguageVersion) Description() string {
	return "Rewrite the language version declaration (go.mod, pyproject.toml, ...) to the rule's target."
}

func (bumpLanguageVersion) Apply(fsys fs.FS, rule spec.Rule, _ model.Finding) ([]model.FileEdit, error) {
	target, err := Target(rule)
	if err != nil {
		return nil, err
	}
	ed, ok := editorFor(rule.Scanner)
	if !ok {
		return nil, fmt.Errorf("recipe: no version editor registered for scanner %q", rule.Scanner)
	}
	return ed.BumpVersion(fsys, target)
}

// bumpBaseImage rewrites the final FROM line of the Dockerfile named by the
// finding to the rule's target image.
type bumpBaseImage struct{}

func (bumpBaseImage) Name() string { return "bump-base-image" }

func (bumpBaseImage) Description() string {
	return "Rewrite the final Dockerfile FROM image to the rule's target."
}

func (bumpBaseImage) Apply(fsys fs.FS, rule spec.Rule, finding model.Finding) ([]model.FileEdit, error) {
	target, err := Target(rule)
	if err != nil {
		return nil, err
	}
	path := finding.Location.File
	if path == "" {
		path = "Dockerfile"
	}
	f, err := fsys.Open(path)
	if err != nil {
		// Nothing to rewrite; a separate exists rule reports the missing file.
		return nil, nil
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	last := -1
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "FROM") {
			last = i
		}
	}
	if last == -1 {
		return nil, fmt.Errorf("recipe: no FROM instruction in %s", path)
	}

	fields := strings.Fields(strings.TrimSpace(lines[last]))
	if fields[1] == target {
		return nil, nil
	}
	fields[1] = target
	lines[last] = strings.Join(fields, " ")

	return []model.FileEdit{{Path: path, Content: []byte(strings.Join(lines, "\n"))}}, nil
}
