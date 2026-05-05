package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/auth"
	"github.com/scovl/ollanta/ollantaweb/config"
)

// AuthHandler handles login, logout, refresh, and OAuth flows.
type AuthHandler struct {
	cfg      *config.Config
	users    *postgres.UserRepository
	groups   *postgres.GroupRepository
	sessions *postgres.SessionRepository
	github   auth.OAuthProvider
	gitlab   auth.OAuthProvider
	google   auth.OAuthProvider
}

// NewAuthHandler creates an AuthHandler wired to the given repositories and config.
func NewAuthHandler(
	cfg *config.Config,
	users *postgres.UserRepository,
	groups *postgres.GroupRepository,
	sessions *postgres.SessionRepository,
) *AuthHandler {
	h := &AuthHandler{cfg: cfg, users: users, groups: groups, sessions: sessions}

	base := cfg.OAuthRedirectBase
	if h.cfg.GitHubClientID != "" {
		h.github = &auth.GitHubProvider{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  base + "/api/v1/auth/github/callback",
		}
	}
	if cfg.GitLabClientID != "" {
		h.gitlab = &auth.GitLabProvider{
			ClientID:     cfg.GitLabClientID,
			ClientSecret: cfg.GitLabClientSecret,
			RedirectURL:  base + "/api/v1/auth/gitlab/callback",
		}
	}
	if cfg.GoogleClientID != "" {
		h.google = &auth.GoogleProvider{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  base + "/api/v1/auth/google/callback",
		}
	}
	return h
}

// loginResponse is the JSON body returned on successful authentication.
type loginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	User         userView `json:"user"`
}

// Login handles POST /api/v1/auth/login with a local login (username) + password.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Login == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "login and password required")
		return
	}

	u, err := h.users.GetByLogin(r.Context(), req.Login)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !u.IsActive {
		jsonError(w, http.StatusUnauthorized, "account deactivated")
		return
	}
	if err := auth.CheckPassword(req.Password, u.PasswordHash); err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	resp, err := h.issueTokenPair(w, r, u)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	_ = h.users.SetLastLogin(r.Context(), u.ID)
	jsonOK(w, http.StatusOK, resp)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		jsonError(w, http.StatusBadRequest, "refresh_token required")
		return
	}

	hash := auth.HashToken(req.RefreshToken)
	sess, err := h.sessions.GetByHash(r.Context(), hash)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = h.sessions.Delete(r.Context(), sess.ID)
		jsonError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	u, err := h.users.GetByID(r.Context(), sess.UserID)
	if err != nil || !u.IsActive {
		jsonError(w, http.StatusUnauthorized, "user not found or deactivated")
		return
	}

	expiry := h.cfg.JWTExpiry
	accessToken, err := auth.GenerateAccessToken([]byte(h.cfg.JWTSecret), u.ID, u.Login, expiry)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not generate token")
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   int(expiry.Seconds()),
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Best-effort: accept either refresh token in body, or just invalidate by user ID.
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		hash := auth.HashToken(req.RefreshToken)
		_ = h.sessions.DeleteByHash(r.Context(), hash)
	}
	w.WriteHeader(http.StatusNoContent)
}

// GitHubRedirect handles GET /api/v1/auth/github.
func (h *AuthHandler) GitHubRedirect(w http.ResponseWriter, r *http.Request) {
	h.oauthRedirect(w, r, h.github, "github")
}

// GitHubCallback handles GET /api/v1/auth/github/callback.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	h.oauthCallback(w, r, h.github, "github")
}

// GitLabRedirect handles GET /api/v1/auth/gitlab.
func (h *AuthHandler) GitLabRedirect(w http.ResponseWriter, r *http.Request) {
	h.oauthRedirect(w, r, h.gitlab, "gitlab")
}

// GitLabCallback handles GET /api/v1/auth/gitlab/callback.
func (h *AuthHandler) GitLabCallback(w http.ResponseWriter, r *http.Request) {
	h.oauthCallback(w, r, h.gitlab, "gitlab")
}

// GoogleRedirect handles GET /api/v1/auth/google.
func (h *AuthHandler) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	h.oauthRedirect(w, r, h.google, "google")
}

// GoogleCallback handles GET /api/v1/auth/google/callback.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	h.oauthCallback(w, r, h.google, "google")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (h *AuthHandler) oauthRedirect(w http.ResponseWriter, r *http.Request, provider auth.OAuthProvider, name string) {
	if provider == nil {
		http.NotFound(w, r)
		return
	}
	state, err := auth.GenerateOAuthState()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not generate state")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state_" + name,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, provider.AuthURL(state), http.StatusFound)
}

func (h *AuthHandler) oauthCallback(w http.ResponseWriter, r *http.Request, provider auth.OAuthProvider, name string) {
	if provider == nil {
		http.NotFound(w, r)
		return
	}

	// Validate CSRF state
	cookie, err := r.Cookie("oauth_state_" + name)
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		jsonError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: "oauth_state_" + name, MaxAge: -1, Path: "/"})

	code := r.URL.Query().Get("code")
	if code == "" {
		jsonError(w, http.StatusBadRequest, "missing code")
		return
	}

	oauthUser, err := provider.Exchange(r.Context(), code)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "oauth exchange failed")
		return
	}

	// Upsert user
	u := &postgres.User{
		Login:      oauthUser.Login,
		Email:      oauthUser.Email,
		Name:       oauthUser.Name,
		AvatarURL:  oauthUser.AvatarURL,
		Provider:   name,
		ProviderID: oauthUser.ProviderID,
	}
	if err := h.users.UpsertOAuth(r.Context(), u); err != nil {
		jsonError(w, http.StatusInternalServerError, "could not upsert user")
		return
	}
	// JIT provisioning: add to default group
	_ = h.groups.AddUserToDefaultGroup(r.Context(), u.ID)

	resp, err := h.issueTokenPair(w, r, u)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	jsonOK(w, http.StatusOK, resp)
}

func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, u *postgres.User) (*loginResponse, error) {
	expiry := h.cfg.JWTExpiry
	accessToken, err := auth.GenerateAccessToken([]byte(h.cfg.JWTSecret), u.ID, u.Login, expiry)
	if err != nil {
		return nil, err
	}

	plain, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr

	sess := &postgres.Session{
		UserID:      u.ID,
		RefreshHash: hash,
		UserAgent:   ua,
		IPAddress:   ip,
		ExpiresAt:   time.Now().Add(h.cfg.RefreshExpiry),
	}
	if err := h.sessions.Create(r.Context(), sess); err != nil {
		return nil, err
	}

	return &loginResponse{
		AccessToken:  accessToken,
		RefreshToken: plain,
		ExpiresIn:    int(expiry.Seconds()),
		User:         toUserView(u),
	}, nil
}
