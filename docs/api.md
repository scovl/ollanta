# Server API Reference

All `/api/v1` routes (except auth) require a `Bearer` token or an API token (`olt_…`) in the `Authorization` header.

The server listens on `:8080` by default. Start it with:

```sh
docker compose --profile server up -d
```

---

## Auth

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

## Core

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/healthz`                                | Liveness probe |
| GET    | `/readyz`                                 | Readiness (PG + Meilisearch) |
| GET    | `/metrics`                                | Prometheus-style metrics |
| POST   | `/api/v1/projects`                        | Create/update a project |
| GET    | `/api/v1/projects`                        | List projects |
| GET    | `/api/v1/projects/{key}`                  | Get project by key |
| DELETE | `/api/v1/projects/{key}`                  | Delete project |
| POST   | `/api/v1/scans`                           | Ingest a scan report |
| GET    | `/api/v1/scans/{id}`                      | Get scan by ID |
| GET    | `/api/v1/projects/{key}/scans`            | List scans for project |
| GET    | `/api/v1/projects/{key}/scans/latest`     | Latest scan for project |
| GET    | `/api/v1/issues`                          | List/filter issues (project, severity, rule, status) |
| GET    | `/api/v1/issues/facets`                   | Issue distribution facets |
| GET    | `/api/v1/projects/{key}/measures/trend`   | Metric trend over time |
| GET    | `/api/v1/search`                          | Full-text search (Meilisearch) |
| POST   | `/admin/reindex`                          | Rebuild Meilisearch from PostgreSQL |

## Users, Groups & Permissions

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/api/v1/users`                           | List users |
| GET    | `/api/v1/users/me`                        | Current user profile |
| POST   | `/api/v1/users`                           | Create user |
| PUT    | `/api/v1/users/{id}`                      | Update user |
| POST   | `/api/v1/users/{id}/change-password`      | Change password |
| DELETE | `/api/v1/users/{id}`                      | Deactivate user |
| GET    | `/api/v1/groups`                          | List groups |
| POST   | `/api/v1/groups`                          | Create group |
| POST   | `/api/v1/groups/{id}/members`             | Add member to group |
| DELETE | `/api/v1/groups/{id}/members/{userID}`    | Remove member |
| DELETE | `/api/v1/groups/{id}`                     | Delete group |
| GET    | `/api/v1/projects/{key}/permissions`      | List project permissions |
| POST   | `/api/v1/projects/{key}/permissions`      | Grant permission |
| DELETE | `/api/v1/projects/{key}/permissions/{id}` | Revoke permission |

## API Tokens

| Method | Endpoint                    | Description |
|--------|-----------------------------|-------------|
| GET    | `/api/v1/tokens`            | List API tokens for current user |
| POST   | `/api/v1/tokens`            | Generate a new API token (`olt_…`) |
| DELETE | `/api/v1/tokens/{id}`       | Revoke token |

## Quality Gates & Profiles

| Method | Endpoint                                           | Description |
|--------|----------------------------------------------------|-------------|
| GET    | `/api/v1/gates`                                    | List quality gates |
| POST   | `/api/v1/gates`                                    | Create quality gate |
| GET    | `/api/v1/gates/{id}`                               | Get gate details |
| PUT    | `/api/v1/gates/{id}`                               | Update gate |
| DELETE | `/api/v1/gates/{id}`                               | Delete gate |
| POST   | `/api/v1/gates/{id}/conditions`                    | Add condition to gate |
| DELETE | `/api/v1/gates/{id}/conditions/{condID}`           | Remove condition |
| GET    | `/api/v1/profiles`                                 | List quality profiles |
| POST   | `/api/v1/profiles`                                 | Create quality profile |
| GET    | `/api/v1/profiles/{id}`                            | Get profile |
| POST   | `/api/v1/profiles/{id}/rules`                      | Activate rule in profile |
| DELETE | `/api/v1/profiles/{id}/rules/{ruleKey}`            | Deactivate rule |
| POST   | `/api/v1/profiles/{id}/import`                     | Import profile from YAML |

## New-code Periods & Webhooks

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/new-code`         | Get new-code period setting |
| PUT    | `/api/v1/projects/{key}/new-code`         | Set new-code period |
| GET    | `/api/v1/projects/{key}/webhooks`         | List webhooks |
| POST   | `/api/v1/projects/{key}/webhooks`         | Register webhook |
| PUT    | `/api/v1/projects/{key}/webhooks/{id}`    | Update webhook |
| DELETE | `/api/v1/projects/{key}/webhooks/{id}`    | Delete webhook |
| GET    | `/api/v1/projects/{key}/webhooks/{id}/deliveries` | List recent deliveries |
