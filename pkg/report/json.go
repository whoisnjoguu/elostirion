package report

import (
	"encoding/json"
	"fmt"
)

type jsonRenderer struct{}

// jsonReport is the stable JSON shape emitted with -format json.
type jsonReport struct {
	Spec   string     `json:"spec,omitempty"`
	Total  int        `json:"total"`
	Repos  []jsonRepo `json:"repos"`
	MaxSev string     `json:"max_severity,omitempty"`
}

type jsonRepo struct {
	Repo     string        `json:"repo"`
	Findings []jsonFinding `json:"findings"`
}

type jsonFinding struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Got      string `json:"got,omitempty"`
	Want     string `json:"want,omitempty"`
}

func (jsonRenderer) Render(r *Report) ([]byte, error) {
	out := jsonReport{Spec: r.SpecName, Total: r.TotalFindings()}
	if sev, ok := r.MaxSeverity(); ok {
		out.MaxSev = string(sev)
	}
	for _, res := range r.Results {
		jr := jsonRepo{Repo: res.Repo.Slug()}
		for _, f := range res.Findings {
			jr.Findings = append(jr.Findings, jsonFinding{
				RuleID:   f.RuleID,
				Severity: string(f.Severity),
				Message:  f.Message,
				File:     f.Location.File,
				Line:     f.Location.Line,
				Got:      f.Got,
				Want:     f.Want,
			})
		}
		out.Repos = append(out.Repos, jr)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("report: marshal json: %w", err)
	}
	return append(data, '\n'), nil
}
