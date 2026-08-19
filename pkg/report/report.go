package report

import (
	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// Report is the outcome of evaluating a spec against one or more repositories.
type Report struct {
	SpecName string
	Results  []RepoResult
}

// RepoResult holds the findings for a single repository.
type RepoResult struct {
	Repo     model.Repo
	Findings []model.Finding
}

// New builds a report for a single repository result.
func New(s *spec.Spec, repo model.Repo, findings []model.Finding) *Report {
	return &Report{
		SpecName: s.Name,
		Results:  []RepoResult{{Repo: repo, Findings: findings}},
	}
}

// Add appends a repository result to the report.
func (r *Report) Add(repo model.Repo, findings []model.Finding) {
	r.Results = append(r.Results, RepoResult{Repo: repo, Findings: findings})
}

// MaxSeverity returns the highest severity present across all findings.
func (r *Report) MaxSeverity() (model.Severity, bool) {
	var max model.Severity
	found := false
	for _, res := range r.Results {
		for _, f := range res.Findings {
			if !found || f.Severity.Rank() > max.Rank() {
				max = f.Severity
				found = true
			}
		}
	}
	return max, found
}

// ExceedsThreshold reports whether any finding meets or exceeds the given severity threshold
func (r *Report) ExceedsThreshold(threshold model.Severity) bool {
	for _, res := range r.Results {
		for _, f := range res.Findings {
			if f.Severity.Rank() >= threshold.Rank() {
				return true
			}
		}
	}
	return false
}

// TotalFindings returns the number of findings across all repositories.
func (r *Report) TotalFindings() int {
	n := 0
	for _, res := range r.Results {
		n += len(res.Findings)
	}
	return n
}

// Renderer writes a report in a specific format.
type Renderer interface {
	Render(r *Report) ([]byte, error)
}

// For returns the renderer for a named format
func For(format string, color bool) (Renderer, error) {
	switch format {
	case "", "text":
		return textRenderer{color: color}, nil
	case "json":
		return jsonRenderer{}, nil
	case "junit":
		return junitRenderer{}, nil
	case "sarif":
		return sarifRenderer{}, nil
	default:
		return nil, errUnknownFormat(format)
	}
}
