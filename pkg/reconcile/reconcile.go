// Package reconcile plans edits that bring repositories into spec compliance.
package reconcile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// Evaluate applies every rule in the spec to the facts and returns the findings for rules that are violated
func Evaluate(s *spec.Spec, facts *model.Facts) []model.Finding {
	var findings []model.Finding
	for _, rule := range s.Rules {
		if rule.Language != "" && !facts.HasLanguage(rule.Language) {
			continue
		}
		if f, ok := evalRule(rule, facts); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// evalRule returns a Finding and true when the rule is violated.
func evalRule(rule spec.Rule, facts *model.Facts) (model.Finding, bool) {
	key := rule.Key()
	raw, present := facts.Get(key)

	// cross-fact rules derive the expected value from another fact
	if rule.ValueFrom != nil {
		ref, ok := facts.Get(rule.ValueFrom.Key)
		if !ok {
			return model.Finding{}, false
		}
		rule.Value = expandValueFrom(rule.ValueFrom.Template, toString(ref))
	}

	switch rule.Op {
	case spec.OpExists:
		if present {
			return model.Finding{}, false
		}
		return violation(rule, facts, "", "present"), true
	case spec.OpAbsent:
		if !present {
			return model.Finding{}, false
		}
		return violation(rule, facts, toString(raw), "absent"), true
	}

	// All remaining operators need the fact to be present.
	if !present {
		return violation(rule, facts, "<missing>", expected(rule)), true
	}

	if list, ok := raw.([]string); ok {
		return evalListRule(rule, facts, list)
	}

	got := toString(raw)
	if rule.IsFileRule() && (rule.Op == spec.OpEquals || rule.Op == spec.OpNotEquals) {
		// Template equality ignores trailing-whitespace noise.
		norm := rule
		norm.Value = normalizeFileContent(rule.Value)
		ok, err := compare(norm, normalizeFileContent(got))
		if err == nil && ok {
			return model.Finding{}, false
		}
		return violation(rule, facts, got, expected(rule)), true
	}
	ok, err := compare(rule, got)
	if err != nil {
		return violation(rule, facts, got, rule.Value), true
	}
	if ok {
		return model.Finding{}, false
	}
	return violation(rule, facts, got, expected(rule)), true
}

// evalListRule applies element-aware semantics when the fact is a list
func evalListRule(rule spec.Rule, facts *model.Facts, list []string) (model.Finding, bool) {
	joined := strings.Join(list, ", ")
	switch rule.Op {
	case spec.OpContains:
		for _, v := range list {
			if v == rule.Value {
				return model.Finding{}, false
			}
		}
		return violation(rule, facts, joined, rule.Value), true
	case spec.OpMatches:
		re, err := regexp.Compile(rule.Value)
		if err != nil {
			return violation(rule, facts, joined, rule.Value), true
		}
		for i, v := range list {
			if !re.MatchString(v) {
				return violationAt(rule, facts.ElemSource(rule.Key(), i), facts, v, rule.Value), true
			}
		}
		return model.Finding{}, false
	case spec.OpNotMatches:
		re, err := regexp.Compile(rule.Value)
		if err != nil {
			return violation(rule, facts, joined, rule.Value), true
		}
		for i, v := range list {
			if re.MatchString(v) {
				return violationAt(rule, facts.ElemSource(rule.Key(), i), facts, v, "not "+rule.Value), true
			}
		}
		return model.Finding{}, false
	case spec.OpOneOf:
		allowed := make(map[string]bool, len(rule.Values))
		for _, v := range rule.Values {
			allowed[v] = true
		}
		for i, v := range list {
			if !allowed[v] {
				return violationAt(rule, facts.ElemSource(rule.Key(), i), facts, v, expected(rule)), true
			}
		}
		return model.Finding{}, false
	case spec.OpSequence:
		i := 0
		for _, v := range list {
			if i < len(rule.Values) && v == rule.Values[i] {
				i++
			}
		}
		if i == len(rule.Values) {
			return model.Finding{}, false
		}
		return violation(rule, facts, joined, expected(rule)), true
	default:
		ok, err := compare(rule, joined)
		if err != nil || !ok {
			return violation(rule, facts, joined, expected(rule)), true
		}
		return model.Finding{}, false
	}
}

// compare returns whether the observed value satisfies the rule.
func compare(rule spec.Rule, got string) (bool, error) {
	switch rule.Op {
	case spec.OpEquals:
		return got == rule.Value, nil
	case spec.OpNotEquals:
		return got != rule.Value, nil
	case spec.OpGTE:
		c, err := compareVersions(got, rule.Value)
		return c >= 0, err
	case spec.OpLTE:
		c, err := compareVersions(got, rule.Value)
		return c <= 0, err
	case spec.OpMatches:
		re, err := regexp.Compile(rule.Value)
		if err != nil {
			return false, err
		}
		return re.MatchString(got), nil
	case spec.OpNotMatches:
		re, err := regexp.Compile(rule.Value)
		if err != nil {
			return false, err
		}
		return !re.MatchString(got), nil
	case spec.OpContains:
		return strings.Contains(got, rule.Value), nil
	case spec.OpOneOf:
		for _, v := range rule.Values {
			if got == v {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown op %q", rule.Op)
	}
}

// violation builds a Finding, applying grace-period downgrade of errors.
func violation(rule spec.Rule, facts *model.Facts, got, want string) model.Finding {
	return violationAt(rule, facts.Sources[rule.Key()], facts, got, want)
}

// violationAt builds a Finding pointing at an explicit location.
func violationAt(rule spec.Rule, loc model.Location, facts *model.Facts, got, want string) model.Finding {
	sev := effectiveSeverity(rule)
	msg := rule.Description
	if msg == "" {
		msg = fmt.Sprintf("%s %s %q", rule.Field, rule.Op, expected(rule))
	}
	return model.Finding{
		RuleID:   rule.ID,
		Message:  msg,
		Severity: sev,
		Repo:     facts.Repo,
		Location: loc,
		Got:      display(got),
		Want:     display(want),
	}
}

// display keeps got/want single-line and bounded for report rendering.
func display(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// effectiveSeverity downgrades an error to a warning while a grace period is still in effect.
func effectiveSeverity(rule spec.Rule) model.Severity {
	if rule.Severity == model.SeverityError && rule.GraceUntil != "" {
		if until, err := time.Parse(time.RFC3339, rule.GraceUntil); err == nil {
			if time.Now().Before(until) {
				return model.SeverityWarn
			}
		} else if d, err := time.Parse("2006-01-02", rule.GraceUntil); err == nil {
			if time.Now().Before(d.Add(24 * time.Hour)) {
				return model.SeverityWarn
			}
		}
	}
	return rule.Severity
}

func expected(rule spec.Rule) string {
	if rule.Op == spec.OpOneOf || rule.Op == spec.OpSequence {
		return strings.Join(rule.Values, ", ")
	}
	return rule.Value
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case []string:
		return strings.Join(t, ", ")
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// expandValueFrom expands a value_from template with the referenced fact's value
func expandValueFrom(template, value string) string {
	if template == "" {
		return value
	}
	out := strings.ReplaceAll(template, "{value.major_minor}", majorMinor(value))
	return strings.ReplaceAll(out, "{value}", value)
}

// majorMinor truncates a dotted version to its first two segments.
func majorMinor(v string) string {
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// compareVersions compares dotted numeric versions such as "1.24" or "1.24.3".
func compareVersions(a, b string) (int, error) {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		var ae, be error
		if i < len(as) {
			ai, ae = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, be = strconv.Atoi(bs[i])
		}
		if ae != nil || be != nil {
			// Fall back to lexical comparison for this component.
			av, bv := "", ""
			if i < len(as) {
				av = as[i]
			}
			if i < len(bs) {
				bv = bs[i]
			}
			if av != bv {
				if av < bv {
					return -1, nil
				}
				return 1, nil
			}
			continue
		}
		if ai != bi {
			if ai < bi {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}
