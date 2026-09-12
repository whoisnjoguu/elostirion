package recipe

import (
	"fmt"
	"io/fs"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

func init() { Register(syncFile{}) }

// syncFile converges a file rule by writing the rule's target
type syncFile struct{}

func (syncFile) Name() string { return "sync-file" }

func (syncFile) Description() string {
	return "Write the rule's target content to the rule's file path."
}

func (syncFile) Apply(fsys fs.FS, rule spec.Rule, _ model.Finding) ([]model.FileEdit, error) {
	if !rule.IsFileRule() {
		return nil, fmt.Errorf("recipe: sync-file requires a file rule (file: set)")
	}
	content, err := Target(rule)
	if err != nil {
		return nil, err
	}
	if data, err := fs.ReadFile(fsys, rule.File); err == nil && string(data) == content {
		return nil, nil // already converged
	}
	return []model.FileEdit{{Path: rule.File, Content: []byte(content)}}, nil
}
