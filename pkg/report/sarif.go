package report

import (
	"encoding/json"
	"fmt"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

type sarifRenderer struct{}

// Minimal SARIF 2.1.0 shapes. GitHub code scanning renders these as inline
// annotations on the pull request diff.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID string `json:"id"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func (sarifRenderer) Render(r *Report) ([]byte, error) {
	run := sarifRun{Tool: sarifTool{Driver: sarifDriver{Name: "elostirion"}}}
	ruleSeen := map[string]bool{}
	for _, res := range r.Results {
		for _, f := range res.Findings {
			if !ruleSeen[f.RuleID] {
				ruleSeen[f.RuleID] = true
				run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{ID: f.RuleID})
			}
			result := sarifResult{
				RuleID:  f.RuleID,
				Level:   sarifLevel(f.Severity),
				Message: sarifMessage{Text: sarifText(f)},
			}
			if f.Location.File != "" {
				loc := sarifLocation{PhysicalLocation: sarifPhysical{
					ArtifactLocation: sarifArtifact{URI: f.Location.File},
				}}
				if f.Location.Line > 0 {
					loc.PhysicalLocation.Region = &sarifRegion{StartLine: f.Location.Line}
				}
				result.Locations = append(result.Locations, loc)
			}
			run.Results = append(run.Results, result)
		}
	}
	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{run},
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("report: marshal sarif: %w", err)
	}
	return append(data, '\n'), nil
}

// sarifText builds the result message, appending got/want only when present.
func sarifText(f model.Finding) string {
	if f.Got == "" && f.Want == "" {
		return f.Message
	}
	return fmt.Sprintf("%s (got %q, want %q)", f.Message, f.Got, f.Want)
}

// sarifLevel maps a severity to a SARIF level. GitHub understands
// error/warning/note.
func sarifLevel(s model.Severity) string {
	switch s {
	case model.SeverityError:
		return "error"
	case model.SeverityDrift, model.SeverityWarn:
		return "warning"
	default:
		return "note"
	}
}
