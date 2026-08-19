package spec

import (
	"fmt"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// Spec is the parsed fleet specification.
type Spec struct {
	Version int    `yaml:"version"`
	Name    string `yaml:"name,omitempty"`
	Rules   []Rule `yaml:"rules"`
}

type Op string

const (
	OpEquals     Op = "eq"       // observed == value
	OpNotEquals  Op = "ne"       // observed != value
	OpGTE        Op = "gte"      // semantic-version / numeric: observed >= value
	OpLTE        Op = "lte"      // semantic-version / numeric: observed <= value
	OpMatches    Op = "matches"  // observed matches regexp value
	OpNotMatches Op = "nmatches" // observed does not match regexp value
	OpExists     Op = "exists"   // fact is present (value ignored)
	OpAbsent     Op = "absent"   // fact is absent (value ignored)
	OpContains   Op = "contains" // observed list/string contains value
	OpOneOf      Op = "oneof"    // observed is one of values
)

// Rule is a single conformance constraint.
type Rule struct {
	ID          string         `yaml:"id"`
	Description string         `yaml:"description,omitempty"`
	Severity    model.Severity `yaml:"severity"`
	Scanner     string         `yaml:"scanner"`
	Field       string         `yaml:"field"`
	Op          Op             `yaml:"op"`
	Value       string         `yaml:"value,omitempty"`
	Values      []string       `yaml:"values,omitempty"`
	GraceUntil  string         `yaml:"grace_until,omitempty"`
	Recipe      string         `yaml:"recipe,omitempty"`
}

// Key returns the "scanner.field" fact key this rule inspects.
func (r Rule) Key() string {
	return r.Scanner + "." + r.Field
}

// Validate checks the spec for structural problems and returns all errors found.
func (s *Spec) Validate() error {
	if s.Version == 0 {
		return fmt.Errorf("spec: missing version")
	}
	if s.Version != 1 {
		return fmt.Errorf("spec: unsupported version %d (want 1)", s.Version)
	}
	if len(s.Rules) == 0 {
		return fmt.Errorf("spec: no rules defined")
	}
	seen := make(map[string]bool, len(s.Rules))
	for i, r := range s.Rules {
		if r.ID == "" {
			return fmt.Errorf("spec: rule %d has no id", i)
		}
		if seen[r.ID] {
			return fmt.Errorf("spec: duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if r.Scanner == "" {
			return fmt.Errorf("spec: rule %q has no scanner", r.ID)
		}
		if r.Field == "" && r.Op != OpExists && r.Op != OpAbsent {
			return fmt.Errorf("spec: rule %q has no field", r.ID)
		}
		if !validOp(r.Op) {
			return fmt.Errorf("spec: rule %q has unknown op %q", r.ID, r.Op)
		}
		if !validSeverity(r.Severity) {
			return fmt.Errorf("spec: rule %q has invalid severity %q", r.ID, r.Severity)
		}
		if r.Op == OpOneOf && len(r.Values) == 0 {
			return fmt.Errorf("spec: rule %q uses op oneof but has no values", r.ID)
		}
	}
	return nil
}

func validOp(op Op) bool {
	switch op {
	case OpEquals, OpNotEquals, OpGTE, OpLTE, OpMatches, OpNotMatches,
		OpExists, OpAbsent, OpContains, OpOneOf:
		return true
	default:
		return false
	}
}

func validSeverity(s model.Severity) bool {
	switch s {
	case model.SeverityError, model.SeverityWarn, model.SeverityDrift, model.SeverityInfo:
		return true
	default:
		return false
	}
}
