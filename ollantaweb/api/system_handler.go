package api

import (
	"net/http"
	"runtime"

	"github.com/scovl/ollanta/ollantacore/constants"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
)

// SystemHandler exposes system information for administrators.
type SystemHandler struct {
	users    *postgres.UserRepository
	projects *postgres.ProjectRepository
	config   *config.Config
}

// Info handles GET /api/v1/system/info — returns system metadata.
// @Summary System info
// @Description Returns system metadata for administrators
// @Tags system
// @Produce json
// @Success 200 {object} systemInfoResponse
// @Router /api/v1/system/info [get]
func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	userCount, err := h.users.Count(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, projectCount, err := h.projects.List(r.Context(), 1, 0)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, map[string]any{
		"version":        constants.Version,
		"go_version":     runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"num_goroutines": runtime.NumGoroutine(),
		"search_backend": h.config.SearchBackend,
		"stats": map[string]any{
			"users":    userCount,
			"projects": projectCount,
		},
	})
}

// UISettings handles GET /api/v1/ui/settings and returns web UI configuration.
// @Summary UI settings
// @Description Returns public UI configuration
// @Tags system
// @Produce json
// @Success 200 {object} uiSettingsResponse
// @Router /api/v1/ui/settings [get]
func (h *SystemHandler) UISettings(w http.ResponseWriter, r *http.Request) {
	links := h.config.ObservabilityLinks
	if links == nil {
		links = []config.ObservabilityLink{}
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"observability_links": links,
	})
}
