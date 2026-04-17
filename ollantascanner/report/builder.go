// Package report assembles scan results into a structured Report and writes
// JSON and SARIF output files to the .ollanta/ directory under the project root.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantascanner/discovery"
)

const Version = "0.1.0"

// Measures holds basic size metrics and issue type counts aggregated across all scanned files.
type Measures struct {
	Files           int            `json:"files"`
	Lines           int            `json:"lines"`
	Ncloc           int            `json:"ncloc"`
	Comments        int            `json:"comments"`
	Bugs            int            `json:"bugs"`
	CodeSmells      int            `json:"code_smells"`
	Vulnerabilities int            `json:"vulnerabilities"`
	ByLang          map[string]int `json:"by_language"` // file count per language
}

// Metadata describes the scan run context.
type Metadata struct {
	ProjectKey   string `json:"project_key"`
	AnalysisDate string `json:"analysis_date"` // RFC 3339
	Version      string `json:"version"`
	ElapsedMs    int64  `json:"elapsed_ms"`
}

// Report is the complete output of a scan run.
type Report struct {
	Metadata Metadata        `json:"metadata"`
	Measures Measures        `json:"measures"`
	Issues   []*domain.Issue `json:"issues"`
}

// Build assembles a Report from the discovered files, analysis results, and elapsed time.
func Build(projectKey string, files []discovery.DiscoveredFile, issues []*domain.Issue, elapsed time.Duration) *Report {
	m := computeMeasures(files)
	for _, iss := range issues {
		switch iss.Type {
		case domain.TypeBug:
			m.Bugs++
		case domain.TypeCodeSmell:
			m.CodeSmells++
		case domain.TypeVulnerability:
			m.Vulnerabilities++
		}
	}
	return &Report{
		Metadata: Metadata{
			ProjectKey:   projectKey,
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
			Version:      Version,
			ElapsedMs:    elapsed.Milliseconds(),
		},
		Measures: m,
		Issues:   issues,
	}
}

// SaveJSON writes the report as pretty-printed JSON to <baseDir>/.ollanta/report.json.
// Returns the path of the file written.
func (r *Report) SaveJSON(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, ".ollanta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create .ollanta dir: %w", err)
	}
	path := filepath.Join(dir, "report.json")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return path, enc.Encode(r)
}

// computeMeasures reads each file to tally line counts and aggregates by language.
func computeMeasures(files []discovery.DiscoveredFile) Measures {
	m := Measures{
		Files:  len(files),
		ByLang: map[string]int{},
	}
	for _, f := range files {
		m.ByLang[f.Language]++
		total, ncloc, comments := countLines(f.Path)
		m.Lines += total
		m.Ncloc += ncloc
		m.Comments += comments
	}
	return m
}

// countLines returns (total lines, ncloc, comment lines) for a file.
// Supports line comments (//, #) and block comments (/* ... */).
// NOTE: detection is line-based and does not track string literals —
// comment markers inside strings will be miscounted.
func countLines(path string) (total, ncloc, comments int) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("ollanta: cannot read %s for metrics: %v", path, err)
		return 0, 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	inBlock := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		total++
		switch {
		case inBlock:
			comments++
			if strings.Contains(line, "*/") {
				inBlock = false
			}
		case strings.HasPrefix(line, "/*"):
			inBlock = true
			comments++
			if strings.Contains(line[2:], "*/") {
				inBlock = false
			}
		case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "#"):
			comments++
		case line == "":
			// blank line — not counted in ncloc or comments
		default:
			ncloc++
		}
	}
	return
}
