// Package newcode resolves the baseline scan for new-code-period analysis.
// It supports five strategies: auto, previous_version, number_of_days,
// specific_analysis, and reference_branch.
package newcode

import (
	"context"
	"regexp"
	"strconv"
	"time"
)

// Strategy is one of the five supported new code period strategies.
type Strategy string

const (
	StrategyAuto             Strategy = "auto"
	StrategyPreviousVersion  Strategy = "previous_version"
	StrategyNumberOfDays     Strategy = "number_of_days"
	StrategySpecificAnalysis Strategy = "specific_analysis"
	StrategyReferenceBranch  Strategy = "reference_branch"
)

// Scan is a minimal representation of a past scan used for baseline resolution.
type Scan struct {
	ID           int64
	Version      string
	Branch       string
	AnalysisDate time.Time
}

// ScanLister is satisfied by any repository that can list scans for a project.
type ScanLister interface {
	// ListByProject returns scans for a project ordered by analysis_date DESC.
	ListByProject(ctx context.Context, projectID int64) ([]Scan, error)
}

// semverPattern matches simple vN.N.N tags (e.g. v1.2.3).
var semverPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// Resolve returns the baseline scan ID for new-code analysis.
// Returns 0 and nil when no baseline exists (first scan).
func Resolve(ctx context.Context, strategy Strategy, value string, projectID int64, lister ScanLister) (int64, error) {
	scans, err := lister.ListByProject(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if len(scans) == 0 {
		return 0, nil
	}

	switch strategy {
	case StrategyAuto:
		return resolveAuto(scans)
	case StrategyPreviousVersion:
		return resolvePreviousVersion(scans)
	case StrategyNumberOfDays:
		return resolveNumberOfDays(scans, value)
	case StrategySpecificAnalysis:
		return resolveSpecificAnalysis(scans, value)
	case StrategyReferenceBranch:
		return resolveReferenceBranch(scans, value)
	default:
		return resolveAuto(scans)
	}
}

// resolveAuto detects a semver tag in scan versions and falls back to 30 days.
func resolveAuto(scans []Scan) (int64, error) {
	// Look for the most recent scan whose version matches a semver tag.
	for _, s := range scans {
		if semverPattern.MatchString(s.Version) {
			return s.ID, nil
		}
	}
	// Fallback: number_of_days=30.
	return resolveNumberOfDays(scans, "30")
}

// resolvePreviousVersion returns the scan immediately before the latest one.
func resolvePreviousVersion(scans []Scan) (int64, error) {
	if len(scans) < 2 {
		return 0, nil
	}
	return scans[1].ID, nil
}

// resolveNumberOfDays returns the oldest scan within N days of the latest scan.
func resolveNumberOfDays(scans []Scan, value string) (int64, error) {
	days, err := strconv.Atoi(value)
	if err != nil || days <= 0 {
		days = 30
	}
	cutoff := scans[0].AnalysisDate.AddDate(0, 0, -days)
	var baseline int64
	for _, s := range scans {
		if s.AnalysisDate.Before(cutoff) {
			break
		}
		baseline = s.ID
	}
	return baseline, nil
}

// resolveSpecificAnalysis returns a scan by its ID (value is the scan ID as string).
func resolveSpecificAnalysis(scans []Scan, value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	for _, s := range scans {
		if s.ID == id {
			return s.ID, nil
		}
	}
	return 0, nil // not found — no baseline
}

// resolveReferenceBranch returns the latest scan on the given branch.
func resolveReferenceBranch(scans []Scan, branch string) (int64, error) {
	for _, s := range scans {
		if s.Branch == branch {
			return s.ID, nil
		}
	}
	return 0, nil
}
