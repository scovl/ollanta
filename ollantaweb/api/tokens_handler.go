package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/auth"
)

// tokenView is the public representation of an API token (no hash).
type tokenView struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	TokenType  string     `json:"token_type"`
	ProjectID  *int64     `json:"project_id,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toTokenView(t *postgres.Token) tokenView {
	return tokenView{
		ID:         t.ID,
		Name:       t.Name,
		TokenType:  t.TokenType,
		ProjectID:  t.ProjectID,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		CreatedAt:  t.CreatedAt,
	}
}

// TokensHandler handles API token CRUD for the authenticated user.
type TokensHandler struct {
	tokens   *postgres.TokenRepository
	projects *postgres.ProjectRepository
	perms    *postgres.PermissionRepository
}

// NewTokensHandler creates a TokensHandler.
func NewTokensHandler(
	tokens *postgres.TokenRepository,
	projects *postgres.ProjectRepository,
	perms *postgres.PermissionRepository,
) *TokensHandler {
	return &TokensHandler{tokens: tokens, projects: projects, perms: perms}
}

// List handles GET /api/v1/users/me/tokens.
// @Summary List my tokens
// @Description Returns API tokens for the authenticated user
// @Tags tokens
// @Produce json
// @Success 200 {object} tokenListResponse
// @Router /api/v1/users/me/tokens [get]
func (h *TokensHandler) List(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	tokens, err := h.tokens.ListByUser(r.Context(), u.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]tokenView, len(tokens))
	for i, t := range tokens {
		views[i] = toTokenView(t)
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{"tokens": views})
}

// Create handles POST /api/v1/users/me/tokens.
// @Summary Create token
// @Description Create a new API token for the authenticated user
// @Tags tokens
// @Accept json
// @Produce json
// @Param body body object{name=string,type=string,project_key=string,expires_in=int} true "Token data"
// @Success 201 {object} tokenCreateResponse
// @Router /api/v1/users/me/tokens [post]
func (h *TokensHandler) Create(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		ProjectKey string `json:"project_key"`
		ExpiresIn  int    `json:"expires_in"` // days; 0 = no expiry
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	switch req.Type {
	case "user", "project_analysis", "global_analysis":
	default:
		jsonError(w, http.StatusBadRequest, "type must be user, project_analysis, or global_analysis")
		return
	}

	tok := &postgres.Token{
		UserID:    u.ID,
		Name:      req.Name,
		TokenType: req.Type,
	}

	if req.Type == "project_analysis" {
		if req.ProjectKey == "" {
			jsonError(w, http.StatusBadRequest, "project_key required for project_analysis tokens")
			return
		}
		proj, err := h.projects.GetByKey(r.Context(), req.ProjectKey)
		if err != nil {
			jsonError(w, http.StatusNotFound, "project not found")
			return
		}
		// Check user has execute_analysis on this project
		ok, err := h.perms.HasProject(r.Context(), u.ID, proj.ID, "execute_analysis")
		if err != nil || !ok {
			jsonError(w, http.StatusForbidden, "missing execute_analysis on project")
			return
		}
		tok.ProjectID = &proj.ID
	}

	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		tok.ExpiresAt = &t
	}

	plain, hash, err := auth.GenerateAPIToken()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not generate token")
		return
	}
	tok.TokenHash = hash

	if err := h.tokens.Create(r.Context(), tok); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the plain-text token only once
	jsonOK(w, http.StatusCreated, map[string]interface{}{
		"token": plain,
		"meta":  toTokenView(tok),
	})
}

// Delete handles DELETE /api/v1/users/me/tokens/{id}.
// @Summary Delete token
// @Description Delete an API token
// @Tags tokens
// @Param id path int true "Token ID"
// @Success 204
// @Router /api/v1/users/me/tokens/{id} [delete]
func (h *TokensHandler) Delete(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid token id")
		return
	}
	if err := h.tokens.Delete(r.Context(), id, u.ID); err != nil {
		jsonError(w, http.StatusNotFound, "token not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
