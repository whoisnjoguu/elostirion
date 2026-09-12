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

// Op is a comparison operator applied by a rule.
type Op string

// Comparison operators supported by rule expressions.
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
	OpSequence   Op = "sequence" // observed list contains values as an ordered subsequence
)

// Rule is a single conformance constraint.
type Rule struct {
	ID          string         `yaml:"id"`
	Description string         `yaml:"description,omitempty"`
	Severity    model.Severity `yaml:"severity"`
	Language    string         `yaml:"language,omitempty"`
	Scanner     string         `yaml:"scanner,omitempty"`
	Field       string         `yaml:"field,omitempty"`
	File        string         `yaml:"file,omitempty"` // file rule: constrain a repo file's presence/content instead of a scanner fact
	Op          Op             `yaml:"op"`
	Value       string         `yaml:"value,omitempty"`
	Values      []string       `yaml:"values,omitempty"`
	ValueFrom   *ValueFrom     `yaml:"value_from,omitempty"` // derive the expected value from another fact
	GraceUntil  string         `yaml:"grace_until,omitempty"`
	Recipe      string         `yaml:"recipe,omitempty"`
	Target      string         `yaml:"target,omitempty"`
}

// ValueFrom derives a rule's expected value from another fact
type ValueFrom struct {
	Key      string `yaml:"key"`                // fact key to read, e.g. gomod.go_version
	Template string `yaml:"template,omitempty"` // expansion with {value} and {value.major_minor}; default {value}
}

// IsFileRule reports whether the rule constrains a file rather than a scanner fact.
func (r Rule) IsFileRule() bool { return r.File != "" }

// Key returns the fact key this rule inspects
func (r Rule) Key() string {
	if r.IsFileRule() {
		return "file:" + r.File
	}
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
		if r.IsFileRule() {
			if r.Scanner != "" || r.Field != "" {
				return fmt.Errorf("spec: rule %q sets both file and scanner/field; use one", r.ID)
			}
			switch r.Op {
			case OpExists, OpAbsent, OpEquals, OpNotEquals, OpMatches, OpNotMatches, OpContains:
			default:
				return fmt.Errorf("spec: file rule %q has unsupported op %q", r.ID, r.Op)
			}
		} else {
			if r.Scanner == "" {
				return fmt.Errorf("spec: rule %q has no scanner", r.ID)
			}
			if r.Field == "" && r.Op != OpExists && r.Op != OpAbsent {
				return fmt.Errorf("spec: rule %q has no field", r.ID)
			}
		}
		if !validOp(r.Op) {
			return fmt.Errorf("spec: rule %q has unknown op %q", r.ID, r.Op)
		}
		if !validSeverity(r.Severity) {
			return fmt.Errorf("spec: rule %q has invalid severity %q", r.ID, r.Severity)
		}
		if (r.Op == OpOneOf || r.Op == OpSequence) && len(r.Values) == 0 {
			return fmt.Errorf("spec: rule %q uses op %s but has no values", r.ID, r.Op)
		}
		if r.ValueFrom != nil {
			if r.ValueFrom.Key == "" {
				return fmt.Errorf("spec: rule %q has value_from without key", r.ID)
			}
			if r.Value != "" {
				return fmt.Errorf("spec: rule %q sets both value and value_from; use one", r.ID)
			}
			switch r.Op {
			case OpExists, OpAbsent, OpOneOf, OpSequence:
				return fmt.Errorf("spec: rule %q op %q cannot use value_from", r.ID, r.Op)
			}
		}
		if r.Recipe != "" && r.Target == "" {
			switch r.Op {
			case OpEquals, OpGTE, OpLTE:
				// value is concrete; usable as the target
			default:
				return fmt.Errorf("spec: rule %q has recipe %q but no target; set target: to the concrete value to converge to", r.ID, r.Recipe)
			}
		}
	}
	return nil
}

func validOp(op Op) bool {
	switch op {
	case OpEquals, OpNotEquals, OpGTE, OpLTE, OpMatches, OpNotMatches,
		OpExists, OpAbsent, OpContains, OpOneOf, OpSequence:
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
