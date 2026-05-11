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

// GoogleProvider implements OAuthProvider for Google.
type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (p *GoogleProvider) AuthURL(state string) string {
	v := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"online"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + v.Encode()
}

func (p *GoogleProvider) Exchange(ctx context.Context, code string) (*OAuthUser, error) {
	body := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {p.RedirectURL},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(b, &tokenResp); err != nil {
		return nil, fmt.Errorf("google token decode: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("google oauth error: %s", tokenResp.Error)
	}

	// Fetch user info
	ureq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	ureq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	uresp, err := http.DefaultClient.Do(ureq)
	if err != nil {
		return nil, fmt.Errorf("google userinfo fetch: %w", err)
	}
	defer uresp.Body.Close()

	var user struct {
		Sub       string `json:"sub"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		GivenName string `json:"given_name"`
		Picture   string `json:"picture"`
	}
	ub, _ := io.ReadAll(io.LimitReader(uresp.Body, 1<<20))
	if err := json.Unmarshal(ub, &user); err != nil {
		return nil, fmt.Errorf("google userinfo decode: %w", err)
	}

	login := "gg_" + strings.ToLower(strings.ReplaceAll(user.GivenName, " ", "_"))
	if login == "gg_" {
		login = "gg_" + user.Sub[:8]
	}

	return &OAuthUser{
		ProviderID: user.Sub,
		Login:      login,
		Email:      user.Email,
		Name:       user.Name,
		AvatarURL:  user.Picture,
	}, nil
}
