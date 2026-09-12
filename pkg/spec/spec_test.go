package spec

import "testing"

func TestParseValid(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Rules) == 0 {
		t.Fatal("default spec has no rules")
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []string{
		"version: 1\nrules: []\n", // no rules
		"version: 2\nrules:\n  - id: a\n    scanner: gomod\n    field: v\n    op: eq\n    severity: error\n",   // bad version
		"version: 1\nrules:\n  - id: a\n    scanner: gomod\n    field: v\n    op: nope\n    severity: error\n", // bad op
		"version: 1\nrules:\n  - id: a\n    scanner: gomod\n    field: v\n    op: eq\n    severity: loud\n",    // bad severity
	}
	for i, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

func TestDuplicateID(t *testing.T) {
	y := "version: 1\nrules:\n" +
		"  - id: a\n    scanner: gomod\n    field: v\n    op: exists\n    severity: error\n" +
		"  - id: a\n    scanner: gomod\n    field: v\n    op: exists\n    severity: error\n"
	if _, err := Parse([]byte(y)); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestFileRuleValidation(t *testing.T) {
	ok := "version: 1\nrules:\n  - id: license\n    file: LICENSE\n    op: exists\n    severity: error\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("valid file rule rejected: %v", err)
	}

	bad := []string{
		// file + scanner both set
		"version: 1\nrules:\n  - id: a\n    file: LICENSE\n    scanner: gomod\n    field: v\n    op: exists\n    severity: error\n",
		// unsupported op for file rules
		"version: 1\nrules:\n  - id: a\n    file: LICENSE\n    op: gte\n    value: \"1\"\n    severity: error\n",
	}
	for i, c := range bad {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("file case %d: expected error, got nil", i)
		}
	}
}

func TestValueFromValidation(t *testing.T) {
	ok := "version: 1\nrules:\n  - id: a\n    scanner: dockerfile\n    field: builder_image\n    op: matches\n    severity: error\n    value_from:\n      key: gomod.go_version\n      template: '^golang:{value.major_minor}'\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("valid value_from rejected: %v", err)
	}

	bad := []string{
		// missing key
		"version: 1\nrules:\n  - id: a\n    scanner: s\n    field: f\n    op: eq\n    severity: error\n    value_from:\n      template: '{value}'\n",
		// both value and value_from
		"version: 1\nrules:\n  - id: a\n    scanner: s\n    field: f\n    op: eq\n    value: x\n    severity: error\n    value_from:\n      key: k.f\n",
		// value_from with exists
		"version: 1\nrules:\n  - id: a\n    scanner: s\n    field: f\n    op: exists\n    severity: error\n    value_from:\n      key: k.f\n",
	}
	for i, c := range bad {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("value_from case %d: expected error, got nil", i)
		}
	}
}

func TestSequenceValidation(t *testing.T) {
	ok := "version: 1\nrules:\n  - id: order\n    scanner: pipeline\n    field: steps\n    op: sequence\n    values: [lint, test]\n    severity: error\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("valid sequence rejected: %v", err)
	}
	// sequence without values
	bad := "version: 1\nrules:\n  - id: order\n    scanner: pipeline\n    field: steps\n    op: sequence\n    severity: error\n"
	if _, err := Parse([]byte(bad)); err == nil {
		t.Error("sequence without values: expected error, got nil")
	}
}
