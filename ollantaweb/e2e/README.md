# Ollanta Web E2E Tests

This directory contains Playwright end-to-end tests for the `ollantaweb` SPA.

## Prerequisites

- Node.js 18+
- The full Ollanta server stack running locally on port **8080**, including:
  - `ollantaweb` (API + SPA)
  - `ollantaworker` (background scan processor)
  - PostgreSQL
  - ZincSearch

Start the stack with:

```bash
docker compose --profile server up -d
```

> **Note:** The `ollantaworker` is required for the issues-related tests because scan ingestion is asynchronous. If the worker is not running, the `issues`, `issue-transitions`, and `project-overview` (issues tab) tests will fail or show empty states.

## Install

```bash
cd ollantaweb/e2e
npm install
npx playwright install chromium
```

## Run

```bash
# Run all tests headless
npm run test:e2e

# Run with UI mode for debugging
npm run test:e2e:ui

# Run a single spec file
npx playwright test tests/auth.spec.ts

# Run with debug
npx playwright test --debug
```

## Global Setup

`global-setup.ts` runs once before all tests:

1. Logs in as the default `admin` / `admin` account.
2. Creates the test project `e2e-test-project`.
3. Submits a scan payload with 3 sample issues.
4. Polls the scan job until the worker marks it **completed**.
5. Saves the JWT + user into `playwright/.auth/user.json` as Playwright `storageState`.

All tests reuse this authenticated state by default.

## Test Coverage

| Spec | Scenarios |
|------|-----------|
| `auth.spec.ts` | Valid login, invalid login (error message), logout |
| `project-list.spec.ts` | Create project via API, verify it appears in the SPA grid |
| `project-overview.spec.ts` | Click a project card, verify tabs (Overview, Issues, Coverage, Activity) render |
| `issues.spec.ts` | Issue list loads, filter by severity facet, click row opens detail panel |
| `issue-transitions.spec.ts` | Mark first issue as false-positive, verify row closes and reopen button appears |
| `command-palette.spec.ts` | `Ctrl+K` opens palette, search works, `Esc` closes it |

## Selectors

Tests rely on existing DOM selectors from the vanilla JS SPA:

- Login: `#loginUser`, `#loginPass`, `#loginBtn`, `#loginError`
- Nav / Logout: `#logoutBtn`
- Project grid: `.project-card[data-key="..."]`
- Project tabs: `.tab-btn[data-tab="..."]`
- Issues table: `.issues-table tbody tr[data-issue-idx]`
- Severity facets: `.facet-option[data-facet-field="severity"]`
- Issue detail: `#detail-panel`, `.detail-title`, `#detailClose`
- Transitions: `.itbtn.fp-btn`, `.itbtn.re-btn`
- Command palette: `#paletteOverlay`, `#paletteInput`

No `data-testid` attributes were added because the existing selectors are explicit and stable.

## Troubleshooting

- **401 on login** → Ensure the server is running and migrations applied (default `admin` user exists).
- **No issues in Issues tab** → Ensure `ollantaworker` is processing scan jobs. Check `docker compose --profile server ps`.
- **Scan job timeout in global setup** → The worker may be down or backlogged. Check worker logs.
