package api

import (
	"encoding/json"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// GatesHandler handles quality gate API endpoints.
type GatesHandler struct {
	gates    *postgres.GateRepository
	projects *postgres.ProjectRepository
}

// NewGatesHandler creates a GatesHandler.
func NewGatesHandler(gates *postgres.GateRepository, projects *postgres.ProjectRepository) *GatesHandler {
	return &GatesHandler{gates: gates, projects: projects}
}

// List handles GET /api/v1/quality-gates
func (h *GatesHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.gates.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// Get handles GET /api/v1/quality-gates/{id}
func (h *GatesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	gate, err := h.gates.GetByID(r.Context(), id)
	if handleNotFound(w, err, "gate not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	conditions, err := h.gates.Conditions(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"gate": gate, "conditions": conditions})
}

// Create handles POST /api/v1/quality-gates
func (h *GatesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var g postgres.QualityGate
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if g.SmallChangesetLines == 0 {
		g.SmallChangesetLines = 20
	}
	if err := h.gates.Create(r.Context(), &g); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Auto-add default conditions (equivalent to SonarQube CAYC)
	for _, c := range defaultGateConditions(g.ID) {
		_ = h.gates.AddCondition(r.Context(), c)
	}
	jsonOK(w, http.StatusCreated, g)
}

func defaultGateConditions(gateID int64) []*postgres.GateCondition {
	return []*postgres.GateCondition{
		{GateID: gateID, Metric: "bugs", Operator: "GT", Threshold: 0, OnNewCode: false},
		{GateID: gateID, Metric: "vulnerabilities", Operator: "GT", Threshold: 0, OnNewCode: false},
		{GateID: gateID, Metric: "new_bugs", Operator: "GT", Threshold: 0, OnNewCode: true},
		{GateID: gateID, Metric: "new_vulnerabilities", Operator: "GT", Threshold: 0, OnNewCode: true},
	}
}

// Update handles PUT /api/v1/quality-gates/{id}
func (h *GatesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var g postgres.QualityGate
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	g.ID = id
	if err := h.gates.Update(r.Context(), &g); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, g)
}

// Delete handles DELETE /api/v1/quality-gates/{id}
func (h *GatesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.gates.Delete(r.Context(), id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddCondition handles POST /api/v1/quality-gates/{id}/conditions
func (h *GatesHandler) AddCondition(w http.ResponseWriter, r *http.Request) {
	gateID, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var c postgres.GateCondition
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	c.GateID = gateID
	if err := h.gates.AddCondition(r.Context(), &c); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, c)
}

// RemoveCondition handles DELETE /api/v1/quality-gates/{id}/conditions/{cid}
func (h *GatesHandler) RemoveCondition(w http.ResponseWriter, r *http.Request) {
	cid, err := parseID(r, "cid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid condition id")
		return
	}
	if err := h.gates.RemoveCondition(r.Context(), cid); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateCondition handles PUT /api/v1/quality-gates/{id}/conditions/{cid}
func (h *GatesHandler) UpdateCondition(w http.ResponseWriter, r *http.Request) {
	cid, err := parseID(r, "cid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid condition id")
		return
	}
	var c postgres.GateCondition
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	c.ID = cid
	if err := h.gates.UpdateCondition(r.Context(), &c); err != nil {
		if handleNotFound(w, err, "condition not found") {
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, c)
}

// Copy handles POST /api/v1/quality-gates/{id}/copy
func (h *GatesHandler) Copy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	gate, err := h.gates.Copy(r.Context(), id, req.Name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, gate)
}

// SetDefault handles POST /api/v1/quality-gates/{id}/set-default
func (h *GatesHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.gates.SetDefault(r.Context(), id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AssignToProject handles POST /api/v1/projects/{key}/quality-gate
func (h *GatesHandler) AssignToProject(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	project, err := h.projects.GetByKey(r.Context(), key)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var req struct {
		GateID int64 `json:"gate_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.gates.AssignToProject(r.Context(), project.ID, req.GateID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
