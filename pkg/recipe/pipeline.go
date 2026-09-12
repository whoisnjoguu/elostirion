package recipe

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

func init() { Register(syncPipeline{}) }

// syncPipeline inserts a named step/job into the pipeline file the finding points at
type syncPipeline struct{}

func (syncPipeline) Name() string { return "sync-pipeline" }

func (syncPipeline) Description() string {
	return "Insert the named pipeline step/job (rule value) with the rule's target YAML body."
}

func (syncPipeline) Apply(fsys fs.FS, rule spec.Rule, finding model.Finding) ([]model.FileEdit, error) {
	name := rule.Value
	if name == "" {
		return nil, fmt.Errorf("recipe: sync-pipeline needs the rule's value to name the step")
	}
	if rule.Target == "" {
		return nil, fmt.Errorf("recipe: sync-pipeline needs target: set to the step's YAML body")
	}
	file := finding.Location.File
	if file == "" {
		return nil, fmt.Errorf("recipe: no pipeline file found to add step %q to", name)
	}

	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("recipe: read %s: %w", file, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("recipe: parse %s: %w", file, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("recipe: %s is not a YAML document", file)
	}
	root := doc.Content[0]

	body, err := parseBody(rule.Target)
	if err != nil {
		return nil, fmt.Errorf("recipe: parse target for rule %q: %w", rule.ID, err)
	}

	switch {
	case path.Base(file) == "bitbucket-pipelines.yml":
		err = insertBitbucketStep(root, name, body)
	case strings.HasPrefix(file, ".github/workflows/"):
		err = insertMapKey(root, "jobs", name, body)
	case path.Base(file) == ".gitlab-ci.yml":
		err = insertTopLevel(root, name, body)
	default:
		return nil, fmt.Errorf("recipe: unrecognized pipeline file %s", file)
	}
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("recipe: render %s: %w", file, err)
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []model.FileEdit{{Path: file, Content: out.Bytes()}}, nil
}

// parseBody parses the rule target into a YAML mapping node.
func parseBody(target string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(target), &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("target must be a YAML mapping (the step body)")
	}
	return doc.Content[0], nil
}

// scalar returns a plain string scalar node.
func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// getOrCreateMap returns the mapping node for key in root, creating it if needed.
func getOrCreateMap(root *yaml.Node, key string, kind yaml.Kind) *yaml.Node {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	n := &yaml.Node{Kind: kind}
	if kind == yaml.MappingNode {
		n.Tag = "!!map"
	} else {
		n.Tag = "!!seq"
	}
	root.Content = append(root.Content, scalar(key), n)
	return n
}

// insertBitbucketStep appends "- step: <body>" under pipelines.default,
// ensuring the body carries the step name.
func insertBitbucketStep(root *yaml.Node, name string, body *yaml.Node) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("bitbucket-pipelines.yml root is not a mapping")
	}
	ensureName(body, name)
	pipelines := getOrCreateMap(root, "pipelines", yaml.MappingNode)
	def := getOrCreateMap(pipelines, "default", yaml.SequenceNode)
	if def.Kind != yaml.SequenceNode {
		return fmt.Errorf("pipelines.default is not a sequence")
	}
	wrapper := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	wrapper.Content = append(wrapper.Content, scalar("step"), body)
	def.Content = append(def.Content, wrapper)
	return nil
}

// insertMapKey adds body under root[section][name] (GitHub workflow jobs).
func insertMapKey(root *yaml.Node, section, name string, body *yaml.Node) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("workflow root is not a mapping")
	}
	jobs := getOrCreateMap(root, section, yaml.MappingNode)
	if jobs.Kind != yaml.MappingNode {
		return fmt.Errorf("%s is not a mapping", section)
	}
	jobs.Content = append(jobs.Content, scalar(name), body)
	return nil
}

// insertTopLevel adds body as a top-level key (GitLab job).
func insertTopLevel(root *yaml.Node, name string, body *yaml.Node) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf(".gitlab-ci.yml root is not a mapping")
	}
	root.Content = append(root.Content, scalar(name), body)
	return nil
}

// ensureName sets name: on the step body when the spec target omitted it.
func ensureName(body *yaml.Node, name string) {
	for i := 0; i+1 < len(body.Content); i += 2 {
		if body.Content[i].Value == "name" {
			return
		}
	}
	body.Content = append([]*yaml.Node{scalar("name"), scalar(name)}, body.Content...)
}
