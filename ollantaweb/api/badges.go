package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// BadgesHandler generates SVG badges for project quality metrics.
// Inspired by SonarQube's api/project_badges — embeddable in READMEs.
type BadgesHandler struct {
	projects *postgres.ProjectRepository
	scans    *postgres.ScanRepository
	measures *postgres.MeasureRepository
}

// QualityGate handles GET /api/v1/projects/{key}/badge?metric=gate_status
// Returns an SVG badge image suitable for embedding in Markdown.
// @Summary Quality badge
// @Description Returns an SVG badge for project quality metrics
// @Tags badges
// @Produce image/svg+xml
// @Param key path string true "Project key"
// @Param metric query string false "Metric (gate_status, bugs, vulnerabilities, code_smells, ncloc, coverage)"
// @Success 200 {string} string "SVG badge"
// @Router /api/v1/projects/{key}/badge [get]
func (h *BadgesHandler) QualityGate(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "gate_status"
	}

	project, err := h.projects.GetByKey(r.Context(), key)
	if errors.Is(err, postgres.ErrNotFound) {
		writeSVG(w, "project", "not found", colorRed)
		return
	}
	if err != nil {
		writeSVG(w, "ollanta", "error", colorRed)
		return
	}

	scan, err := h.scans.GetLatest(r.Context(), project.ID)
	if errors.Is(err, postgres.ErrNotFound) {
		writeSVG(w, metric, "no data", colorGray)
		return
	}
	if err != nil {
		writeSVG(w, "ollanta", "error", colorRed)
		return
	}

	switch metric {
	case "gate_status":
		renderGateBadge(w, scan.GateStatus)
	case "bugs":
		writeSVG(w, "bugs", fmt.Sprintf("%d", scan.TotalBugs), metricColor(scan.TotalBugs))
	case "vulnerabilities":
		writeSVG(w, "vulnerabilities", fmt.Sprintf("%d", scan.TotalVulnerabilities), metricColor(scan.TotalVulnerabilities))
	case "code_smells":
		writeSVG(w, "code smells", fmt.Sprintf("%d", scan.TotalCodeSmells), metricColor(scan.TotalCodeSmells))
	case "ncloc":
		writeSVG(w, "lines of code", formatCount(scan.TotalNcloc), colorBlue)
	case "coverage":
		h.renderCoverageBadge(w, r.Context(), project.ID)
	default:
		writeSVG(w, metric, "unknown", colorGray)
	}
}

func renderGateBadge(w http.ResponseWriter, status string) {
	label := "quality gate"
	color := colorGreen
	switch status {
	case "OK", "PASSED":
		status = "passed"
	case "ERROR", "FAILED":
		status = "failed"
		color = colorRed
	case "WARN":
		status = "warning"
		color = colorYellow
	default:
		status = "none"
		color = colorGray
	}
	writeSVG(w, label, status, color)
}

func (h *BadgesHandler) renderCoverageBadge(w http.ResponseWriter, ctx context.Context, projectID int64) {
	m, err := h.measures.GetLatest(ctx, projectID, "coverage")
	if err != nil {
		writeSVG(w, "coverage", "N/A", colorGray)
		return
	}
	writeSVG(w, "coverage", fmt.Sprintf("%.1f%%", m.Value), coverageColor(m.Value))
}

// ─── SVG generation ────────────────────────────────────────────────────────────

const (
	colorGreen  = "#4c1"
	colorRed    = "#e05d44"
	colorYellow = "#dfb317"
	colorGray   = "#9f9f9f"
	colorBlue   = "#007ec6"
	colorOrange = "#fe7d37"
)

func metricColor(count int) string {
	switch {
	case count == 0:
		return colorGreen
	case count <= 5:
		return colorYellow
	case count <= 20:
		return colorOrange
	default:
		return colorRed
	}
}

func coverageColor(pct float64) string {
	switch {
	case pct >= 80:
		return colorGreen
	case pct >= 60:
		return colorYellow
	case pct >= 40:
		return colorOrange
	default:
		return colorRed
	}
}

func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func writeSVG(w http.ResponseWriter, label, value, color string) {
	// Shield-style badge inspired by shields.io / SonarQube badges
	labelWidth := len(label)*7 + 10
	valueWidth := len(value)*7 + 10
	totalWidth := labelWidth + valueWidth

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img">
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="20" fill="#555"/>
    <rect x="%d" width="%d" height="20" fill="%s"/>
    <rect width="%d" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14" fill="#fff">%s</text>
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14" fill="#fff">%s</text>
  </g>
</svg>`,
		totalWidth, totalWidth,
		labelWidth, labelWidth, valueWidth, color,
		totalWidth,
		labelWidth/2, label,
		labelWidth/2, label,
		labelWidth+valueWidth/2, value,
		labelWidth+valueWidth/2, value,
	)
}
