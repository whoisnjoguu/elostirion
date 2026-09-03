package spec

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and validates a spec from a source
func Load(source string) (*Spec, error) {
	switch {
	case strings.HasPrefix(source, "git::"):
		return loadGit(source)
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		return loadURL(source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s: %w", source, err)
	}
	return Parse(data)
}

// Parse decodes and validates a spec from YAML bytes.
func Parse(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("spec: parse yaml: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Marshal renders a spec back to canonical YAML.
func Marshal(s *Spec) ([]byte, error) {
	return yaml.Marshal(s)
}
