package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GitLabProvider implements OAuthProvider for GitLab.com (or self-hosted via BaseURL).
type GitLabProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	BaseURL      string // defaults to https://gitlab.com
}

func (p *GitLabProvider) base() string {
	if p.BaseURL != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return "https://gitlab.com"
}

func (p *GitLabProvider) AuthURL(state string) string {
	v := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURL},
		"response_type": {"code"},
		"scope":         {"read_user"},
		"state":         {state},
	}
	return p.base() + "/oauth/authorize?" + v.Encode()
}

func (p *GitLabProvider) Exchange(ctx context.Context, code string) (*OAuthUser, error) {
	body := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {p.RedirectURL},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		p.base()+"/oauth/token", strings.NewReader(body.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab token exchange: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(b, &tokenResp); err != nil {
		return nil, fmt.Errorf("gitlab token decode: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("gitlab oauth error: %s", tokenResp.Error)
	}

	// Fetch user
	ureq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		p.base()+"/api/v4/user", nil)
	ureq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	uresp, err := http.DefaultClient.Do(ureq)
	if err != nil {
		return nil, fmt.Errorf("gitlab user fetch: %w", err)
	}
	defer uresp.Body.Close()

	var user struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	ub, _ := io.ReadAll(io.LimitReader(uresp.Body, 1<<20))
	if err := json.Unmarshal(ub, &user); err != nil {
		return nil, fmt.Errorf("gitlab user decode: %w", err)
	}

	return &OAuthUser{
		ProviderID: fmt.Sprintf("%d", user.ID),
		Login:      "gl_" + user.Username,
		Email:      user.Email,
		Name:       user.Name,
		AvatarURL:  user.AvatarURL,
	}, nil
}
