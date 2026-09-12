package reconcile

import (
	"io/fs"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// EvaluateFS collects file facts for the spec's file rules from fsys
func EvaluateFS(s *spec.Spec, facts *model.Facts, fsys fs.FS) []model.Finding {
	CollectFileFacts(fsys, s, facts)
	return Evaluate(s, facts)
}

// CollectFileFacts reads each file named by a file rule and records its content
// under the rule's "file:<path>" key
func CollectFileFacts(fsys fs.FS, s *spec.Spec, facts *model.Facts) {
	if fsys == nil {
		return
	}
	for _, r := range s.Rules {
		if !r.IsFileRule() {
			continue
		}
		key := r.Key()
		if _, ok := facts.Get(key); ok {
			continue // another rule already read this file
		}
		data, err := fs.ReadFile(fsys, r.File)
		if err != nil {
			continue
		}
		facts.Set(key, string(data), model.Location{File: r.File})
	}
}

// normalizeFileContent trims trailing whitespace per line and at EOF
func normalizeFileContent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
