package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// OverviewHandler serves the project overview (dashboard) endpoint.
// Inspired by SonarQube's api/navigation/component — a single call
// that returns everything the frontend needs to render the project dashboard.
type OverviewHandler struct {
	projects *postgres.ProjectRepository
	scans    *postgres.ScanRepository
	issues   *postgres.IssueRepository
	measures *postgres.MeasureRepository
	scanJobs *postgres.ScanJobRepository
	gates    *postgres.GateRepository
	periods  *postgres.NewCodePeriodRepository
}

// overviewResponse is the single-call dashboard payload.
type overviewResponse struct {
	Project     *postgres.Project     `json:"project"`
	Scope       *scopeResponse        `json:"scope,omitempty"`
	LastScan    *postgres.Scan        `json:"last_scan,omitempty"`
	QualityGate *overviewGate         `json:"quality_gate,omitempty"`
	Measures    map[string]float64    `json:"measures"`
	Facets      *postgres.IssueFacets `json:"facets,omitempty"`
	NewCode     *overviewNewCode      `json:"new_code,omitempty"`
	Summary     *overviewSummary      `json:"summary,omitempty"`
}

type overviewGate struct {
	Status     string                    `json:"status"`
	Conditions []*postgres.GateCondition `json:"conditions,omitempty"`
}

type overviewNewCode struct {
	NewIssues    int `json:"new_issues"`
	ClosedIssues int `json:"closed_issues"`
}

func (h *OverviewHandler) resolveOverviewScope(r *http.Request) (*resolvedProjectScope, error) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		return nil, err
	}
	return resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
}

func (h *OverviewHandler) loadOverviewScan(ctx context.Context, resolved *resolvedProjectScope) (*postgres.Scan, error) {
	if resolved == nil {
		return nil, nil
	}
	return h.scans.GetLatestInScope(ctx, resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
}

func (h *OverviewHandler) applyOverviewScan(ctx context.Context, resp *overviewResponse, scan *postgres.Scan) error {
	if scan == nil {
		return nil
	}
	resp.LastScan = scan
	resp.NewCode = &overviewNewCode{NewIssues: scan.NewIssues, ClosedIssues: scan.ClosedIssues}
	facets, err := h.issues.Facets(ctx, scan.ProjectID, scan.ID)
	if err == nil {
		resp.Facets = facets
	}
	return nil
}

func (h *OverviewHandler) loadOverviewGate(ctx context.Context, projectID int64, scan *postgres.Scan) *overviewGate {
	gate, conds, err := h.gates.ForProject(ctx, projectID)
	if err != nil || gate == nil {
		return nil
	}
	status := "NONE"
	if scan != nil && scan.GateStatus != "" {
		status = scan.GateStatus
	}
	return &overviewGate{Status: status, Conditions: conds}
}

func (h *OverviewHandler) fillOverviewMeasures(ctx context.Context, resp *overviewResponse, scan *postgres.Scan) {
	if resp == nil {
		return
	}
	metricKeys := []string{
		"files", "lines", "ncloc", "comments",
		"bugs", "code_smells", "vulnerabilities",
		"coverage", "duplicated_lines_density",
	}
	for _, mk := range metricKeys {
		if scan == nil {
			continue
		}
		measure, err := h.measures.GetForScan(ctx, scan.ID, mk)
		if err == nil && measure != nil {
			resp.Measures[mk] = measure.Value
		}
	}
	if scan == nil {
		return
	}
	if _, ok := resp.Measures["files"]; !ok {
		resp.Measures["files"] = float64(scan.TotalFiles)
	}
	if _, ok := resp.Measures["lines"]; !ok {
		resp.Measures["lines"] = float64(scan.TotalLines)
	}
	if _, ok := resp.Measures["ncloc"]; !ok {
		resp.Measures["ncloc"] = float64(scan.TotalNcloc)
	}
	if _, ok := resp.Measures["bugs"]; !ok {
		resp.Measures["bugs"] = float64(scan.TotalBugs)
	}
	if _, ok := resp.Measures["code_smells"]; !ok {
		resp.Measures["code_smells"] = float64(scan.TotalCodeSmells)
	}
	if _, ok := resp.Measures["vulnerabilities"]; !ok {
		resp.Measures["vulnerabilities"] = float64(scan.TotalVulnerabilities)
	}
}

// Overview handles GET /api/v1/projects/{key}/overview.
//
// Returns the project dashboard in a single response: project metadata,
// latest scan, quality gate status, key measures, issue facets, and
// new code summary. Modelled after SonarQube's unified dashboard call.
func (h *OverviewHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resolved, err := h.resolveOverviewScope(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, projectNotFoundMessage)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := overviewResponse{
		Project:  resolved.Project,
		Scope:    toScopeResponse(resolved),
		Measures: make(map[string]float64),
	}

	scan, err := h.loadOverviewScan(ctx, resolved)
	if err != nil && !errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if scan != nil {
		if err := h.applyOverviewScan(ctx, &resp, scan); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	resp.QualityGate = h.loadOverviewGate(ctx, resolved.Project.ID, scan)
	h.fillOverviewMeasures(ctx, &resp, scan)
	resp.Summary, err = h.loadOverviewSummary(ctx, resolved, scan, &resp)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, resp)
}
