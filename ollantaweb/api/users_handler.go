package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/auth"
)

// userView is the public representation of a user (no password hash).
type userView struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
	IsActive  bool   `json:"is_active"`
}

func toUserView(u *postgres.User) userView {
	return userView{
		ID:        u.ID,
		Login:     u.Login,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
		Provider:  u.Provider,
		IsActive:  u.IsActive,
	}
}

// UsersHandler handles CRUD for users.
type UsersHandler struct {
	users  *postgres.UserRepository
	tokens *postgres.TokenRepository
}

// NewUsersHandler creates a UsersHandler.
func NewUsersHandler(users *postgres.UserRepository, tokens *postgres.TokenRepository) *UsersHandler {
	return &UsersHandler{users: users, tokens: tokens}
}

// Me handles GET /api/v1/users/me.
// @Summary Current user
// @Description Returns the currently authenticated user
// @Tags users
// @Produce json
// @Success 200 {object} userView
// @Router /api/v1/users/me [get]
func (h *UsersHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	jsonOK(w, http.StatusOK, toUserView(u))
}

// List handles GET /api/v1/users (requires manage_users).
// @Summary List users
// @Description Returns paginated list of users
// @Tags users
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} userListResponse
// @Router /api/v1/users [get]
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	users, total, err := h.users.List(r.Context(), page, size)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]userView, len(users))
	for i, u := range users {
		views[i] = toUserView(u)
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"users": views,
		"total": total,
	})
}

// Get handles GET /api/v1/users/{id} (requires manage_users).
// @Summary Get user
// @Description Returns a single user by ID
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} userView
// @Router /api/v1/users/{id} [get]
func (h *UsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	jsonOK(w, http.StatusOK, toUserView(u))
}

// Create handles POST /api/v1/users (requires manage_users).
// @Summary Create user
// @Description Create a new local user
// @Tags users
// @Accept json
// @Produce json
// @Param body body object{login=string,email=string,name=string,password=string} true "User data"
// @Success 201 {object} userView
// @Router /api/v1/users [post]
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Login == "" || req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "login, email, and password are required")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	u := &postgres.User{
		Login:        req.Login,
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
		Provider:     "local",
	}
	if err := h.users.Create(r.Context(), u); err != nil {
		jsonError(w, http.StatusConflict, "login or email already exists")
		return
	}
	jsonOK(w, http.StatusCreated, toUserView(u))
}

// Update handles PUT /api/v1/users/{id} (requires manage_users).
// @Summary Update user
// @Description Update user name, email, or avatar
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param body body object{name=string,email=string,avatar_url=string} true "User data"
// @Success 200 {object} userView
// @Router /api/v1/users/{id} [put]
func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	if req.AvatarURL != "" {
		u.AvatarURL = req.AvatarURL
	}
	if err := h.users.Update(r.Context(), u); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, toUserView(u))
}

// Deactivate handles DELETE /api/v1/users/{id} (requires manage_users).
// @Summary Deactivate user
// @Description Deactivate a user account
// @Tags users
// @Param id path int true "User ID"
// @Success 204
// @Router /api/v1/users/{id} [delete]
func (h *UsersHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.users.Deactivate(r.Context(), id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reactivate handles POST /api/v1/users/{id}/reactivate (requires manage_users).
// @Summary Reactivate user
// @Description Reactivate a deactivated user account
// @Tags users
// @Param id path int true "User ID"
// @Success 204
// @Router /api/v1/users/{id}/reactivate [post]
func (h *UsersHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.users.Reactivate(r.Context(), id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword handles PUT /api/v1/users/me/password (self-service).
// @Summary Change password
// @Description Change the current user's password
// @Tags users
// @Accept json
// @Param body body object{old_password=string,new_password=string} true "Password data"
// @Success 204
// @Router /api/v1/users/me/password [put]
func (h *UsersHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		jsonError(w, http.StatusBadRequest, "old_password and new_password are required")
		return
	}
	if err := auth.CheckPassword(req.OldPassword, u.PasswordHash); err != nil {
		jsonError(w, http.StatusForbidden, "old password is incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	if err := h.users.SetPassword(r.Context(), u.ID, hash); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTokens handles GET /api/v1/users/{id}/tokens (requires manage_users).
// @Summary List user tokens
// @Description List API tokens for a specific user
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} tokenListResponse
// @Router /api/v1/users/{id}/tokens [get]
func (h *UsersHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	tokens, err := h.tokens.ListByUser(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{"tokens": tokens})
}

// DeleteToken handles DELETE /api/v1/users/{id}/tokens/{tid} (requires manage_users).
// @Summary Delete user token
// @Description Delete a specific token for a user
// @Tags users
// @Param id path int true "User ID"
// @Param tid path int true "Token ID"
// @Success 204
// @Router /api/v1/users/{id}/tokens/{tid} [delete]
func (h *UsersHandler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	userID, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	tokenID, err := parseID(r, "tid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid token id")
		return
	}
	if err := h.tokens.Delete(r.Context(), tokenID, userID); err != nil {
		jsonError(w, http.StatusNotFound, "token not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
