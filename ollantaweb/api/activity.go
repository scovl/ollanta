package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// ActivityHandler serves the project activity timeline.
// Inspired by SonarQube's api/project_analyses/search — each scan becomes
// an activity entry decorated with notable events (quality gate changes,
// version bumps, issue spikes).
type ActivityHandler struct {
	scans    *postgres.ScanRepository
	projects *postgres.ProjectRepository
	measures *postgres.MeasureRepository
}

type activityEntry struct {
	ScanID               int64              `json:"scan_id"`
	AnalysisDate         time.Time          `json:"analysis_date"`
	Version              string             `json:"version,omitempty"`
	Branch               string             `json:"branch,omitempty"`
	CommitSHA            string             `json:"commit_sha,omitempty"`
	GateStatus           string             `json:"gate_status"`
	ElapsedMs            int64              `json:"elapsed_ms,omitempty"`
	TotalFiles           int                `json:"total_files"`
	TotalNcloc           int                `json:"total_ncloc"`
	TotalIssues          int                `json:"total_issues"`
	TotalBugs            int                `json:"total_bugs"`
	TotalCodeSmells      int                `json:"total_code_smells"`
	TotalVulnerabilities int                `json:"total_vulnerabilities"`
	NewIssues            int                `json:"new_issues"`
	ClosedIssues         int                `json:"closed_issues"`
	Delta                activityDelta      `json:"delta"`
	Measures             map[string]float64 `json:"measures,omitempty"`
	Events               []activityEvent    `json:"events"`
}

type activityDelta struct {
	Issues          int `json:"issues"`
	Bugs            int `json:"bugs"`
	CodeSmells      int `json:"code_smells"`
	Vulnerabilities int `json:"vulnerabilities"`
	Ncloc           int `json:"ncloc"`
}

type activityEvent struct {
	Category string `json:"category"` // "QUALITY_GATE", "VERSION", "ISSUE_SPIKE", "FIRST_ANALYSIS"
	Name     string `json:"name"`
	Value    string `json:"value,omitempty"`
}

func appendScanComparisonEvents(entry *activityEntry, current, previous *postgres.Scan) {
	if entry == nil || current == nil || previous == nil {
		return
	}
	entry.Delta = activityDelta{
		Issues:          current.TotalIssues - previous.TotalIssues,
		Bugs:            current.TotalBugs - previous.TotalBugs,
		CodeSmells:      current.TotalCodeSmells - previous.TotalCodeSmells,
		Vulnerabilities: current.TotalVulnerabilities - previous.TotalVulnerabilities,
		Ncloc:           current.TotalNcloc - previous.TotalNcloc,
	}
	if current.GateStatus != previous.GateStatus && current.GateStatus != "" {
		entry.Events = append(entry.Events, activityEvent{
			Category: "QUALITY_GATE",
			Name:     "Quality Gate " + current.GateStatus,
			Value:    previous.GateStatus + " → " + current.GateStatus,
		})
	}
	if current.Version != previous.Version && current.Version != "" {
		entry.Events = append(entry.Events, activityEvent{Category: "VERSION", Name: "Version " + current.Version, Value: current.Version})
	}
	if previous.TotalIssues <= 0 || current.NewIssues <= 0 {
		return
	}
	increase := float64(current.NewIssues) / float64(previous.TotalIssues)
	if increase > 0.5 {
		entry.Events = append(entry.Events, activityEvent{Category: "ISSUE_SPIKE", Name: "Issue spike detected", Value: strconv.Itoa(current.NewIssues) + " new issues"})
	}
}

func buildActivityEntry(scan *postgres.Scan) activityEntry {
	return activityEntry{
		ScanID:               scan.ID,
		AnalysisDate:         scan.AnalysisDate,
		Version:              scan.Version,
		Branch:               scan.Branch,
		CommitSHA:            scan.CommitSHA,
		GateStatus:           scan.GateStatus,
		ElapsedMs:            scan.ElapsedMs,
		TotalFiles:           scan.TotalFiles,
		TotalNcloc:           scan.TotalNcloc,
		TotalIssues:          scan.TotalIssues,
		TotalBugs:            scan.TotalBugs,
		TotalCodeSmells:      scan.TotalCodeSmells,
		TotalVulnerabilities: scan.TotalVulnerabilities,
		NewIssues:            scan.NewIssues,
		ClosedIssues:         scan.ClosedIssues,
	}
}

var activityMeasureKeys = []string{
	model.MetricCoverage,
	model.MetricMutationScore,
	model.MetricChangedMutationScore,
	model.MetricTestFailures,
	model.MetricTestErrors,
}

func (h *ActivityHandler) decorateActivityMeasures(ctx context.Context, entries []activityEntry) {
	if h.measures == nil {
		return
	}
	for i := range entries {
		for _, metricKey := range activityMeasureKeys {
			measure, err := h.measures.GetForScan(ctx, entries[i].ScanID, metricKey)
			if errors.Is(err, postgres.ErrNotFound) {
				continue
			}
			if err != nil {
				continue
			}
			if entries[i].Measures == nil {
				entries[i].Measures = map[string]float64{}
			}
			entries[i].Measures[metricKey] = measure.Value
		}
	}
}

func buildActivityEntries(scans []*postgres.Scan, total, offset, limit int) []activityEntry {
	entries := make([]activityEntry, 0, len(scans))
	for i, s := range scans {
		if i >= limit {
			break
		}
		entry := buildActivityEntry(s)

		if offset+i == total-1 {
			entry.Events = append(entry.Events, activityEvent{Category: "FIRST_ANALYSIS", Name: "First analysis"})
		}
		if i+1 < len(scans) {
			appendScanComparisonEvents(&entry, s, scans[i+1])
		}
		entries = append(entries, entry)
	}
	return entries
}

// Activity handles GET /api/v1/projects/{key}/activity?limit=20&offset=0
//
// Returns a chronological timeline of scans with notable events highlighted.
// Events are derived by comparing consecutive scans (quality gate changes,
// version bumps, issue spikes).
func (h *ActivityHandler) Activity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	resolved, err := resolveProjectScope(ctx, h.projects, h.scans, routeParam(r, "key"), requested)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, projectNotFoundMessage)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	allScans, err := h.scans.ListByProjectInScope(ctx, resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total := len(allScans)
	if offset > total {
		offset = total
	}
	end := offset + limit + 1
	if end > total {
		end = total
	}
	scans := allScans[offset:end]

	entries := buildActivityEntries(scans, total, offset, limit)
	h.decorateActivityMeasures(ctx, entries)

	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  entries,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"scope":  toScopeResponse(resolved),
	})
}
