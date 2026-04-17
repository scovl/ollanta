package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// sarifLog is the root object of a SARIF 2.1.0 log file.
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
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	ShortDescription sarifMessage    `json:"shortDescription"`
	DefaultConfig    sarifRuleConfig `json:"defaultConfiguration"`
}

type sarifRuleConfig struct {
	Level string `json:"level"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine,omitempty"`
}

var severityLevel = map[string]string{
	"blocker":  "error",
	"critical": "error",
	"major":    "error",
	"minor":    "warning",
	"info":     "note",
}

// toSARIF converts a Report to a SARIF 2.1.0 log.
func toSARIF(r *Report) *sarifLog {
	seen := map[string]bool{}
	var rules []sarifRule
	for _, iss := range r.Issues {
		if seen[iss.RuleKey] {
			continue
		}
		seen[iss.RuleKey] = true
		lvl := severityLevel[string(iss.Severity)]
		if lvl == "" {
			lvl = "warning"
		}
		rules = append(rules, sarifRule{
			ID:               iss.RuleKey,
			Name:             iss.RuleKey,
			ShortDescription: sarifMessage{Text: iss.RuleKey},
			DefaultConfig:    sarifRuleConfig{Level: lvl},
		})
	}

	results := make([]sarifResult, 0, len(r.Issues))
	for _, iss := range r.Issues {
		lvl := severityLevel[string(iss.Severity)]
		if lvl == "" {
			lvl = "warning"
		}
		endLine := iss.EndLine
		if endLine == 0 {
			endLine = iss.Line
		}
		results = append(results, sarifResult{
			RuleID:  iss.RuleKey,
			Level:   lvl,
			Message: sarifMessage{Text: iss.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: iss.ComponentPath},
					Region:           sarifRegion{StartLine: iss.Line, EndLine: endLine},
				},
			}},
		})
	}

	return &sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "Ollanta",
				Version:        Version,
				InformationURI: "https://github.com/scovl/ollanta",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
}

// SaveSARIF writes the report as SARIF 2.1.0 to <baseDir>/.ollanta/report.sarif.
// Returns the path of the file written.
func (r *Report) SaveSARIF(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, ".ollanta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create .ollanta dir: %w", err)
	}
	path := filepath.Join(dir, "report.sarif")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return path, enc.Encode(toSARIF(r))
}
