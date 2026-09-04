package report

import (
	"encoding/xml"
	"fmt"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

type junitRenderer struct{}

// JUnit XML shapes. Each rule evaluated against a repo becomes a testcase so
type junitTestsuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Suites   []junitTestsuite `xml:"testsuite"`
}

type junitTestsuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Cases    []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

func (junitRenderer) Render(r *Report) ([]byte, error) {
	suites := junitTestsuites{Name: nonEmpty(r.SpecName, "elostirion")}
	for _, res := range r.Results {
		suite := junitTestsuite{Name: res.Repo.Slug()}
		for _, f := range res.Findings {
			tc := junitTestcase{
				Name:      f.RuleID,
				Classname: res.Repo.Slug(),
				Failure: &junitFailure{
					Message: f.Message,
					Type:    string(f.Severity),
					Text:    junitDetail(f),
				},
			}
			suite.Cases = append(suite.Cases, tc)
			suite.Failures++
		}
		suite.Tests = len(suite.Cases)
		suites.Failures += suite.Failures
		suites.Tests += suite.Tests
		suites.Suites = append(suites.Suites, suite)
	}
	data, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("report: marshal junit: %w", err)
	}
	return append([]byte(xml.Header), append(data, '\n')...), nil
}

func junitDetail(f model.Finding) string {
	s := f.RuleID
	if f.Got != "" || f.Want != "" {
		s += fmt.Sprintf(": got %q, want %q", f.Got, f.Want)
	}
	if loc := locString(f.Location); loc != "" {
		s += fmt.Sprintf(" (%s)", loc)
	}
	return s
}

// locString renders a location as file:line, file, or empty.
func locString(l model.Location) string {
	if l.File == "" {
		return ""
	}
	if l.Line > 0 {
		return fmt.Sprintf("%s:%d", l.File, l.Line)
	}
	return l.File
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
