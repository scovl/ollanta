package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

// GitHubProvider implements port.IOAuthProvider for GitHub.
type GitHubProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// compile-time check
var _ port.IOAuthProvider = (*GitHubProvider)(nil)

func (p *GitHubProvider) AuthURL(state string) string {
	v := url.Values{
		"client_id":    {p.ClientID},
		"redirect_uri": {p.RedirectURL},
		"scope":        {"user:email"},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + v.Encode()
}

// githubPrimaryEmail fetches the verified primary email from the /user/emails endpoint.
// Returns empty string on any error or if no matching address is found.
func githubPrimaryEmail(ctx context.Context, token string) string {
	emails, err := githubAPIGet[[]struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}](ctx, "https://api.github.com/user/emails", token)
	if err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	return ""
}

func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*model.OAuthUser, error) {
	v := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.RedirectURL},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(v.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github token exchange: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("github token decode: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s", tokenResp.Error)
	}

	user, err := githubAPIGet[struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}](ctx, "https://api.github.com/user", tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("github user fetch: %w", err)
	}

	email := user.Email
	if email == "" {
		email = githubPrimaryEmail(ctx, tokenResp.AccessToken)
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	return &model.OAuthUser{
		ID: fmt.Sprintf("%d", user.ID),
		Login:      "gh_" + user.Login,
		Email:      email,
		Name:       name,
		AvatarURL:  user.AvatarURL,
	}, nil
}

func githubAPIGet[T any](ctx context.Context, apiURL, token string) (T, error) {
	var zero T
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, err
	}
	if resp.StatusCode >= 400 {
		return zero, fmt.Errorf("github API %s: %s", apiURL, resp.Status)
	}
	var result T
	return result, json.Unmarshal(body, &result)
}
