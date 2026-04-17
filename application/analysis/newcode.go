// Package analysis resolves the baseline scan for new-code-period analysis.
// It supports five strategies: auto, previous_version, number_of_days,
// specific_analysis, and reference_branch.
package analysis

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
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

// semverPattern matches simple vN.N.N tags (e.g. v1.2.3).
var semverPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// ResolveNewCode returns the baseline scan ID for new-code analysis.
// Returns 0 and nil when no baseline exists (first scan).
// lister must satisfy port.ScanLister (IScanRepo satisfies it automatically).
func ResolveNewCode(ctx context.Context, strategy Strategy, value string, projectID int64, lister port.ScanLister) (int64, error) {
	scanPtrs, err := lister.ListByProject(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if len(scanPtrs) == 0 {
		return 0, nil
	}

	// Flatten to value slice for internal helpers.
	scans := make([]model.Scan, len(scanPtrs))
	for i, s := range scanPtrs {
		scans[i] = *s
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
func resolveAuto(scans []model.Scan) (int64, error) {
	for _, s := range scans {
		if semverPattern.MatchString(s.Version) {
			return s.ID, nil
		}
	}
	return resolveNumberOfDays(scans, "30")
}

// resolvePreviousVersion returns the scan immediately before the latest one.
func resolvePreviousVersion(scans []model.Scan) (int64, error) {
	if len(scans) < 2 {
		return 0, nil
	}
	return scans[1].ID, nil
}

// resolveNumberOfDays returns the oldest scan within N days of the latest scan.
func resolveNumberOfDays(scans []model.Scan, value string) (int64, error) {
	days, err := strconv.Atoi(value)
	if err != nil || days <= 0 {
		days = 30
	}
	cutoff := scans[0].AnalysisDate.Add(-time.Duration(days) * 24 * time.Hour)
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
func resolveSpecificAnalysis(scans []model.Scan, value string) (int64, error) {
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
func resolveReferenceBranch(scans []model.Scan, branch string) (int64, error) {
	for _, s := range scans {
		if s.Branch == branch {
			return s.ID, nil
		}
	}
	return 0, nil
}
