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
// @Summary List quality gates
// @Description Returns all quality gates
// @Tags quality-gates
// @Produce json
// @Success 200 {array} postgres.QualityGate
// @Router /api/v1/quality-gates [get]
func (h *GatesHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.gates.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// Get handles GET /api/v1/quality-gates/{id}
// @Summary Get quality gate
// @Description Returns a quality gate with its conditions
// @Tags quality-gates
// @Produce json
// @Param id path int true "Gate ID"
// @Success 200 {object} gateDetailResponse
// @Router /api/v1/quality-gates/{id} [get]
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
// @Summary Create quality gate
// @Description Create a new quality gate with default conditions
// @Tags quality-gates
// @Accept json
// @Produce json
// @Param body body postgres.QualityGate true "Gate data"
// @Success 201 {object} postgres.QualityGate
// @Router /api/v1/quality-gates [post]
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
// @Summary Update quality gate
// @Description Update a quality gate
// @Tags quality-gates
// @Accept json
// @Produce json
// @Param id path int true "Gate ID"
// @Param body body postgres.QualityGate true "Gate data"
// @Success 200 {object} postgres.QualityGate
// @Router /api/v1/quality-gates/{id} [put]
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
// @Summary Delete quality gate
// @Description Delete a quality gate
// @Tags quality-gates
// @Param id path int true "Gate ID"
// @Success 204
// @Router /api/v1/quality-gates/{id} [delete]
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
// @Summary Add condition
// @Description Add a condition to a quality gate
// @Tags quality-gates
// @Accept json
// @Produce json
// @Param id path int true "Gate ID"
// @Param body body postgres.GateCondition true "Condition data"
// @Success 201 {object} postgres.GateCondition
// @Router /api/v1/quality-gates/{id}/conditions [post]
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
// @Summary Remove condition
// @Description Remove a condition from a quality gate
// @Tags quality-gates
// @Param id path int true "Gate ID"
// @Param cid path int true "Condition ID"
// @Success 204
// @Router /api/v1/quality-gates/{id}/conditions/{cid} [delete]
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
// @Summary Update condition
// @Description Update a quality gate condition
// @Tags quality-gates
// @Accept json
// @Produce json
// @Param id path int true "Gate ID"
// @Param cid path int true "Condition ID"
// @Param body body postgres.GateCondition true "Condition data"
// @Success 200 {object} postgres.GateCondition
// @Router /api/v1/quality-gates/{id}/conditions/{cid} [put]
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
// @Summary Copy quality gate
// @Description Copy an existing quality gate
// @Tags quality-gates
// @Accept json
// @Produce json
// @Param id path int true "Gate ID"
// @Param body body object{name=string} true "Copy data"
// @Success 201 {object} postgres.QualityGate
// @Router /api/v1/quality-gates/{id}/copy [post]
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
// @Summary Set default gate
// @Description Set a quality gate as the default
// @Tags quality-gates
// @Param id path int true "Gate ID"
// @Success 204
// @Router /api/v1/quality-gates/{id}/set-default [post]
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
// @Summary Assign quality gate to project
// @Description Assign a quality gate to a project
// @Tags quality-gates
// @Accept json
// @Param key path string true "Project key"
// @Param body body object{gate_id=int64} true "Assignment data"
// @Success 204
// @Router /api/v1/projects/{key}/quality-gate [post]
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
