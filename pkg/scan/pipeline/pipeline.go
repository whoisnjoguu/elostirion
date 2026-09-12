package pipeline

import (
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

func init() { scan.Register(Scanner{}) }

// Scanner extracts the pipeline fact namespace.
type Scanner struct{}

// Name returns the fact namespace.
func (Scanner) Name() string { return "pipeline" }

// Language is empty: pipeline files are language-agnostic.
func (Scanner) Language() string { return "" }

// Markers let a pipeline-only repository be discovered.
func (Scanner) Markers() []string { return []string{"bitbucket-pipelines.yml", ".gitlab-ci.yml"} }

// entry is one named step/job or image occurrence with its position.
type entry struct {
	value string
	loc   model.Location
}

// Scan reads every recognized pipeline file and records merged facts
func (Scanner) Scan(fsys fs.FS, facts *model.Facts) error {
	var providers []entry
	var steps, images []entry

	if doc, ok := parseFile(fsys, "bitbucket-pipelines.yml"); ok {
		providers = append(providers, entry{"bitbucket", model.Location{File: "bitbucket-pipelines.yml", Line: 1}})
		s, i := scanBitbucket(doc, "bitbucket-pipelines.yml")
		steps, images = append(steps, s...), append(images, i...)
	}
	if ghFiles := workflowFiles(fsys); len(ghFiles) > 0 {
		providers = append(providers, entry{"github", model.Location{File: ghFiles[0], Line: 1}})
		for _, wf := range ghFiles {
			if doc, ok := parseFile(fsys, wf); ok {
				s, i := scanGitHub(doc, wf)
				steps, images = append(steps, s...), append(images, i...)
			}
		}
	}
	if doc, ok := parseFile(fsys, ".gitlab-ci.yml"); ok {
		providers = append(providers, entry{"gitlab", model.Location{File: ".gitlab-ci.yml", Line: 1}})
		s, i := scanGitLab(doc, ".gitlab-ci.yml")
		steps, images = append(steps, s...), append(images, i...)
	}

	if len(providers) == 0 {
		return nil
	}

	facts.Set("pipeline.provider", providers[0].value, providers[0].loc)
	setList(facts, "pipeline.providers", providers)
	setList(facts, "pipeline.steps", steps)
	setList(facts, "pipeline.images", dedupe(images))
	facts.Set("pipeline.step_count", len(steps), providers[0].loc)
	return nil
}

// setList stores entries as a list fact with per-element locations.
func setList(facts *model.Facts, key string, entries []entry) {
	values := make([]string, len(entries))
	locs := make([]model.Location, len(entries))
	for i, e := range entries {
		values[i], locs[i] = e.value, e.loc
	}
	facts.SetList(key, values, locs)
}

// dedupe keeps the first occurrence of each value.
func dedupe(entries []entry) []entry {
	seen := map[string]bool{}
	out := entries[:0]
	for _, e := range entries {
		if !seen[e.value] {
			seen[e.value] = true
			out = append(out, e)
		}
	}
	return out
}

// workflowFiles lists .github/workflows/*.yml|yaml sorted by name.
func workflowFiles(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, ".github/workflows")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := path.Ext(e.Name()); ext == ".yml" || ext == ".yaml" {
			out = append(out, path.Join(".github/workflows", e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// parseFile parses a YAML file into its root mapping node.
func parseFile(fsys fs.FS, name string) (*yaml.Node, bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, false
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, false
	}
	return doc.Content[0], true
}

// mapGet returns the value node for key in a mapping node.
func mapGet(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// mapKeys iterates the key/value pairs of a mapping node.
func mapKeys(n *yaml.Node, fn func(k, v *yaml.Node)) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		fn(n.Content[i], n.Content[i+1])
	}
}

// scanBitbucket walks every step block under pipelines
func scanBitbucket(root *yaml.Node, file string) (steps, images []entry) {
	if img := mapGet(root, "image"); img != nil && img.Value != "" {
		images = append(images, entry{img.Value, model.Location{File: file, Line: img.Line}})
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.SequenceNode:
			for _, c := range n.Content {
				walk(c)
			}
		case yaml.MappingNode:
			mapKeys(n, func(k, v *yaml.Node) {
				switch k.Value {
				case "step":
					if name := mapGet(v, "name"); name != nil {
						steps = append(steps, entry{name.Value, model.Location{File: file, Line: name.Line}})
					}
					if img := mapGet(v, "image"); img != nil && img.Value != "" {
						images = append(images, entry{img.Value, model.Location{File: file, Line: img.Line}})
					}
				default:
					walk(v)
				}
			})
		}
	}
	if p := mapGet(root, "pipelines"); p != nil {
		walk(p)
	}
	return steps, images
}

// scanGitHub records job ids and container/service images of one workflow file.
func scanGitHub(root *yaml.Node, file string) (steps, images []entry) {
	mapKeys(mapGet(root, "jobs"), func(id, job *yaml.Node) {
		steps = append(steps, entry{id.Value, model.Location{File: file, Line: id.Line}})
		if c := mapGet(job, "container"); c != nil {
			images = append(images, imageOf(c, file)...)
		}
		mapKeys(mapGet(job, "services"), func(_, svc *yaml.Node) {
			images = append(images, imageOf(svc, file)...)
		})
	})
	return steps, images
}

// imageOf extracts an image from a string node or a mapping's image key.
func imageOf(n *yaml.Node, file string) []entry {
	if n.Kind == yaml.ScalarNode && n.Value != "" {
		return []entry{{n.Value, model.Location{File: file, Line: n.Line}}}
	}
	if img := mapGet(n, "image"); img != nil && img.Value != "" {
		return []entry{{img.Value, model.Location{File: file, Line: img.Line}}}
	}
	return nil
}

// gitlabReserved are top-level .gitlab-ci.yml keys that are not jobs.
var gitlabReserved = map[string]bool{
	"image": true, "services": true, "stages": true, "types": true,
	"before_script": true, "after_script": true, "variables": true,
	"cache": true, "include": true, "workflow": true, "default": true,
}

// scanGitLab records job names (non-reserved, non-hidden top-level keys) and images.
func scanGitLab(root *yaml.Node, file string) (steps, images []entry) {
	addImage := func(n *yaml.Node) {
		if n != nil {
			images = append(images, imageOf(n, file)...)
		}
	}
	addImage(mapGet(root, "image"))
	addImage(mapGet(mapGet(root, "default"), "image"))
	mapKeys(root, func(k, v *yaml.Node) {
		if gitlabReserved[k.Value] || strings.HasPrefix(k.Value, ".") {
			return
		}
		if v.Kind != yaml.MappingNode { // e.g. a stray scalar; not a job
			return
		}
		steps = append(steps, entry{k.Value, model.Location{File: file, Line: k.Line}})
		addImage(mapGet(v, "image"))
	})
	return steps, images
}
