# Authentication

The server supports three authentication mechanisms:

| Mechanism    | How to use |
|--------------|------------|
| **Local**    | `POST /api/v1/auth/login` with `login` + `password`; returns a short-lived JWT and a refresh token |
| **OAuth**    | GitHub, GitLab, or Google — configure via environment variables below; users are auto-provisioned on first login |
| **API tokens** | Generate via `POST /api/v1/tokens`; prefix `olt_`; use as `Authorization: Bearer olt_…` in CI scripts |

## OAuth Environment Variables

| Variable | Description |
|----------|-------------|
| `OLLANTA_GITHUB_CLIENT_ID` / `_SECRET` | GitHub OAuth app credentials |
| `OLLANTA_GITLAB_CLIENT_ID` / `_SECRET` | GitLab OAuth app credentials |
| `OLLANTA_GOOGLE_CLIENT_ID` / `_SECRET` | Google OAuth app credentials |
| `OLLANTA_OAUTH_REDIRECT_BASE` | External base URL for OAuth callbacks (e.g. `https://ollanta.example.com`) |
| `OLLANTA_JWT_SECRET` | HMAC-SHA256 signing key (random if unset — tokens won't survive restarts) |
| `OLLANTA_JWT_EXPIRY` | Access token lifetime, e.g. `15m` (default) |

## Auth API Endpoints

| Method | Endpoint                        | Description |
|--------|---------------------------------|-------------|
| POST   | `/api/v1/auth/login`            | Email+password login → JWT + refresh token |
| POST   | `/api/v1/auth/refresh`          | Refresh access token |
| POST   | `/api/v1/auth/logout`           | Invalidate refresh token |
| GET    | `/api/v1/auth/github`           | Start GitHub OAuth flow |
| GET    | `/api/v1/auth/github/callback`  | GitHub OAuth callback |
| GET    | `/api/v1/auth/gitlab`           | Start GitLab OAuth flow |
| GET    | `/api/v1/auth/gitlab/callback`  | GitLab OAuth callback |
| GET    | `/api/v1/auth/google`           | Start Google OAuth flow |
| GET    | `/api/v1/auth/google/callback`  | Google OAuth callback |

## API Token Endpoints

| Method | Endpoint             | Description |
|--------|----------------------|-------------|
| GET    | `/api/v1/tokens`     | List API tokens for current user |
| POST   | `/api/v1/tokens`     | Generate a new API token (`olt_…`) |
| DELETE | `/api/v1/tokens/{id}`| Revoke token |
