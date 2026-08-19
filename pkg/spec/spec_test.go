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
