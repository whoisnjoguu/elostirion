package diff

import (
	"fmt"
	"io/fs"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// Unified renders a unified diff between old and new content of one file
func Unified(path string, oldContent, newContent []byte) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), string(oldContent), string(newContent))
	return fmt.Sprint(gotextdiff.ToUnified("a/"+path, "b/"+path, string(oldContent), edits))
}

// ForPlan renders the unified diff of every edit in a plan against the current contents of fsys
func ForPlan(fsys fs.FS, plan model.ChangePlan) string {
	var out string
	for _, e := range plan.Edits {
		var oldContent []byte
		if f, err := fsys.Open(e.Path); err == nil {
			buf := make([]byte, 0, 4096)
			tmp := make([]byte, 4096)
			for {
				n, err := f.Read(tmp)
				if n > 0 {
					buf = append(buf, tmp[:n]...)
				}
				if err != nil {
					break
				}
			}
			_ = f.Close()
			oldContent = buf
		}
		out += Unified(e.Path, oldContent, e.Content)
	}
	return out
}
