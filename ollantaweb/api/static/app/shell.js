'use strict';

import { apiFetch, setUnauthorizedHandler } from './core/api.js';
import { parseProjectRoute } from './core/scope.js';
import { createInitialState, replaceState, resetProjectState, state } from './core/state.js';
import { clearStorage, getToken, loadUser, saveToken, saveUser } from './core/storage.js';
import { badgeClassForGateStatus, cardClassForGateStatus, escAttr, escHtml, fmtDate } from './core/utils.js';
import { closeIssueDetail } from './features/issues.js';
import { bindBackgroundTasksContent, loadBackgroundTasksData, renderBackgroundTasksPage } from './features/admin.js';
import { bindProjectViewControls, loadProject, renderProjectDetail } from './project-flow.js';

const BRAND_MARK_PATH = '/branding/ollanta-mark.png';

const CORE_OBSERVABILITY_LINKS = [
  { label: 'Metrics', href: '/metrics' },
];

const API_DOC_SECTIONS = [
  {
    title: 'Authentication',
    endpoints: [
      { method: 'POST', path: '/api/v1/auth/login', permission: 'Public', description: 'Exchange local credentials for an access token and refresh token.' },
      { method: 'POST', path: '/api/v1/auth/refresh', permission: 'Public', description: 'Refresh an access token from a valid refresh token.' },
      { method: 'POST', path: '/api/v1/auth/logout', permission: 'Authenticated', description: 'Invalidate the current session.' },
      { method: 'GET', path: '/api/v1/users/me', permission: 'Authenticated', description: 'Read the current user profile.' },
      { method: 'GET', path: '/api/v1/users/me/tokens', permission: 'Authenticated', description: 'List personal API tokens.' },
      { method: 'POST', path: '/api/v1/users/me/tokens', permission: 'Authenticated', description: 'Create a personal API token.' },
    ],
  },
  {
    title: 'Projects And Scans',
    endpoints: [
      { method: 'GET', path: '/api/v1/projects', permission: 'Authenticated', description: 'List projects visible to the current user.' },
      { method: 'POST', path: '/api/v1/projects', permission: 'create_project', description: 'Create or register a project.' },
      { method: 'GET', path: '/api/v1/projects/{key}', permission: 'Authenticated', description: 'Read project metadata.' },
      { method: 'POST', path: '/api/v1/scans', permission: 'Scanner token or execute_analysis', description: 'Ingest a scanner report into the server.' },
      { method: 'GET', path: '/api/v1/projects/{key}/overview', permission: 'Authenticated', description: 'Read overview metrics, gate status, review summary, and issue signals.' },
      { method: 'GET', path: '/api/v1/projects/{key}/activity', permission: 'Authenticated', description: 'Read project activity over scans.' },
    ],
  },
  {
    title: 'Issues, Code, And Measures',
    endpoints: [
      { method: 'GET', path: '/api/v1/projects/{key}/issues', permission: 'Authenticated', description: 'Search and filter issues by quality, severity, status, lifecycle, language, rule, tag, directory, and file.' },
      { method: 'GET', path: '/api/v1/projects/{key}/code/tree', permission: 'Authenticated', description: 'Browse the indexed source tree for a project scope.' },
      { method: 'GET', path: '/api/v1/projects/{key}/code/file', permission: 'Authenticated', description: 'Read stored source file content with inline issue metadata.' },
      { method: 'GET', path: '/api/v1/projects/{key}/measures/trend', permission: 'Authenticated', description: 'Read time-series measure trends.' },
      { method: 'GET', path: '/api/v1/search', permission: 'Authenticated', description: 'Search indexed source and issue content.' },
    ],
  },
  {
    title: 'Administration',
    endpoints: [
      { method: 'GET', path: '/api/v1/system/info', permission: 'admin', description: 'Read runtime metadata and system counts.' },
      { method: 'GET', path: '/api/v1/permissions/global', permission: 'admin', description: 'List global permission grants.' },
      { method: 'GET', path: '/api/v1/projects/{key}/permissions', permission: 'admin', description: 'List project permission grants.' },
      { method: 'GET', path: '/api/v1/admin/index-jobs', permission: 'admin', description: 'Inspect durable index projection jobs.' },
      { method: 'GET', path: '/api/v1/admin/webhook-jobs', permission: 'admin', description: 'Inspect durable webhook delivery jobs.' },
      { method: 'GET', path: '/api/v1/admin/background-tasks', permission: 'admin', description: 'List normalized scan, index, and webhook background tasks with operational filters.' },
      { method: 'GET', path: '/api/v1/admin/background-tasks/summary', permission: 'admin', description: 'Read queue depth, stale, retry, failure, and worker lag indicators.' },
      { method: 'GET', path: '/api/v1/ui/settings', permission: 'Authenticated', description: 'Read UI settings such as optional external observability links.' },
    ],
  },
  {
    title: 'Public Health And Observability',
    endpoints: [
      { method: 'GET', path: '/healthz', permission: 'Public', description: 'Liveness probe.' },
      { method: 'GET', path: '/readyz', permission: 'Public', description: 'Readiness probe with database and search checks.' },
      { method: 'GET', path: '/metrics', permission: 'Public', description: 'Prometheus-compatible metrics endpoint.' },
    ],
  },
];

setUnauthorizedHandler(logout);

export function render() {
  const app = document.getElementById('app');
  if (state.view === 'login') {
    app.innerHTML = renderLogin();
    bindLogin();
    return;
  }
  app.innerHTML = renderNav() + '<main>' + renderContent() + '</main>';
  bindMain();
}

function renderNav() {
  const user = state.user || {};
  const name = user.name || user.login || 'User';
  return `<nav>
    ${renderBrandLockup()}
    ${renderAdminNavLinks()}
    <span class="spacer"></span>
    <span class="user-info">${escHtml(name)}</span>
    <button class="logout-btn" id="logoutBtn">Sign out</button>
  </nav>`;
}

function renderAdminNavLinks() {
  const configuredLinks = state.uiSettings?.observabilityLinks || [];
  const links = CORE_OBSERVABILITY_LINKS.concat(configuredLinks);
  return `<div class="admin-nav-links" aria-label="Admin shortcuts">
    <button class="admin-nav-link" type="button" id="apiDocsBtn">API</button>
    <button class="admin-nav-link" type="button" id="backgroundTasksBtn">Background Tasks</button>
    ${links.map(renderAdminNavLink).join('')}
  </div>`;
}

function renderAdminNavLink(link) {
  return `<a class="admin-nav-link" href="${escAttr(link.href)}" target="_blank" rel="noopener noreferrer">${escHtml(link.label)}</a>`;
}

function renderBrandLockup() {
  return `<span class="brand-lockup brand-lockup-inline">
    <img class="brand-mark" src="${BRAND_MARK_PATH}" alt="Ollanta" width="36" height="36">
  </span>`;
}



function renderContent() {
  if (state.view === 'projects') return renderDashboard();
  if (state.view === 'project') return renderProjectDetail();
  if (state.view === 'api-docs') return renderApiDocsPage();
  if (state.view === 'background-tasks') return renderBackgroundTasksPage();
  return '';
}

export function renderApiDocsPage() {
  return `<div class="api-docs-page">
    <div class="page-header api-docs-header">
      <div>
        <h2>Ollanta API</h2>
        <p>HTTP API reference for automation, integrations, administration, and observability.</p>
      </div>
      <button class="back-btn" id="apiDocsBackBtn">Back to projects</button>
    </div>
    <div class="api-docs-note">
      Authenticated API calls must send <span class="mono">Authorization: Bearer &lt;token&gt;</span>. Scanner ingestion can also use the configured scanner token.
    </div>
    <div class="api-docs-grid">
      ${API_DOC_SECTIONS.map(renderApiDocSection).join('')}
    </div>
  </div>`;
}

function renderApiDocSection(section) {
  return `<section class="api-docs-section">
    <h3>${escHtml(section.title)}</h3>
    <div class="api-endpoint-list">
      ${section.endpoints.map(renderApiEndpoint).join('')}
    </div>
  </section>`;
}

function renderApiEndpoint(endpoint) {
  const example = buildApiEndpointExample(endpoint);
  return `<details class="api-endpoint-row">
    <summary class="api-endpoint-summary">
      <span class="api-method api-method-${escAttr(endpoint.method.toLowerCase())}">${escHtml(endpoint.method)}</span>
      <span class="api-path">${escHtml(endpoint.path)}</span>
      <span class="api-permission">${escHtml(endpoint.permission)}</span>
      <span class="api-example-toggle">Example</span>
      <p>${escHtml(endpoint.description)}</p>
    </summary>
    <div class="api-example-panel">
      <p>${escHtml(example.description)}</p>
      <pre><code>${escHtml(example.command)}</code></pre>
    </div>
  </details>`;
}

function buildApiEndpointExample(endpoint) {
  const baseURL = endpoint.path.startsWith('/api/') ? '$OLLANTA_URL' : '$OLLANTA_PUBLIC_URL';
  const examplePath = endpoint.examplePath || endpoint.path;
  const lines = [`curl -X ${endpoint.method} "${baseURL}${examplePath}"`];

  if (endpoint.permission !== 'Public') {
    lines.push('  -H "Authorization: Bearer $OLLANTA_TOKEN"');
  }
  if (endpoint.method !== 'GET') {
    lines.push('  -H "Content-Type: application/json"');
  }
  const body = endpoint.exampleBody || defaultExampleBody(endpoint);
  if (body) {
    lines.push(`  -d '${JSON.stringify(body, null, 2)}'`);
  }

  return {
    description: endpoint.exampleDescription || 'Generic request example. Replace placeholders such as {key}, file paths, and tokens with values from your environment.',
    command: lines.join(' \\\n'),
  };
}

function defaultExampleBody(endpoint) {
  if (endpoint.path === '/api/v1/auth/login') return { login: 'admin', password: 'admin' };
  if (endpoint.path === '/api/v1/auth/refresh') return { refresh_token: '$OLLANTA_REFRESH_TOKEN' };
  if (endpoint.path === '/api/v1/auth/logout') return {};
  if (endpoint.path === '/api/v1/users/me/tokens') return { name: 'ci-token', expires_at: '2026-12-31T23:59:59Z' };
  if (endpoint.path === '/api/v1/projects') return { key: 'demo', name: 'Demo Project' };
  if (endpoint.path === '/api/v1/scans') {
    return {
      project_key: 'demo',
      branch: 'main',
      analysis_date: '2026-05-01T12:00:00Z',
      issues: [],
      measures: {},
    };
  }
  return null;
}

function renderLogin() {
  return `<div class="login-wrapper">
    <div class="login-card-unified">
      <div class="login-card-header">
        <img class="login-brand-mark" src="${BRAND_MARK_PATH}" alt="" aria-hidden="true" width="80" height="80">
        <div class="login-brand-text">
          <span class="login-brand-name">Ollanta</span>
          <span class="login-brand-tagline">Static Analysis Platform</span>
        </div>
        <div class="login-features">
          <div class="login-feature">
            <span class="login-feature-icon">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            </span>
            <span>Quality gates &amp; metrics across scans</span>
          </div>
          <div class="login-feature">
            <span class="login-feature-icon">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>
            </span>
            <span>Multi-project, branch &amp; PR tracking</span>
          </div>
          <div class="login-feature">
            <span class="login-feature-icon">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>
            </span>
            <span>Code browsing with inline issue markers</span>
          </div>
        </div>
      </div>
      <div class="login-card-content">
        <p class="login-eyebrow">Welcome back</p>
        <h1 class="login-title">Sign in</h1>
        <div class="field">
          <label for="loginUser">Username</label>
          <input id="loginUser" type="text" placeholder="admin" autocomplete="username">
        </div>
        <div class="field">
          <label for="loginPass">Password</label>
          <input id="loginPass" type="password" placeholder="\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" autocomplete="current-password">
        </div>
        <button class="btn btn-primary" id="loginBtn">Sign in</button>
        <div id="loginError" class="error-msg"></div>
      </div>
    </div>
  </div>`;
}

function bindLogin() {
  const btn = document.getElementById('loginBtn');
  const errEl = document.getElementById('loginError');
  const userEl = document.getElementById('loginUser');
  const passEl = document.getElementById('loginPass');

  async function doLogin() {
    const login = userEl.value.trim();
    const password = passEl.value;
    if (!login || !password) {
      errEl.textContent = 'Enter username and password.';
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Signing in\u2026';
    errEl.textContent = '';

    try {
      const data = await apiFetch('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ login, password }),
      });
      saveToken(data.access_token);
      saveUser(data.user || {});
      state.user = data.user || {};
      await loadProjects();
    } catch (err) {
      errEl.textContent = err.message || 'Login failed.';
      btn.disabled = false;
      btn.textContent = 'Sign in';
    }
  }

  btn.addEventListener('click', doLogin);
  passEl.addEventListener('keydown', event => { if (event.key === 'Enter') doLogin(); });
  userEl.addEventListener('keydown', event => { if (event.key === 'Enter') passEl.focus(); });
}

export async function loadProjects() {
  resetProjectState();
  state.view = 'projects';
  state.loading = true;
  history.replaceState({}, '', globalThis.location.pathname);
  render();

  await loadUISettings();

  try {
    const data = await apiFetch('/projects?limit=100');
    state.projects = data.items || [];
  } catch {
    state.projects = [];
  }

  state.loading = false;
  render();
}

async function loadUISettings() {
  try {
    const data = await apiFetch('/ui/settings');
    state.uiSettings = { observabilityLinks: normalizeObservabilityLinks(data.observability_links) };
  } catch {
    state.uiSettings = { observabilityLinks: [] };
  }
}

function normalizeObservabilityLinks(items) {
  if (!Array.isArray(items)) return [];
  return items
    .map(item => ({ label: String(item?.label || '').trim(), href: String(item?.url || '').trim() }))
    .filter(item => item.label && item.href);
}

function renderDashboard() {
  if (state.loading) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  const projects = state.projects;
  const count = projects.length;

  return `
    <div class="page-header">
      <h2>Projects <span style="font-size:14px;color:var(--text-muted);font-weight:400">(${count})</span></h2>
      <p>All projects registered on this platform</p>
    </div>
    ${count === 0
      ? `<div class="empty-state">
           <div class="empty-icon">\uD83D\uDCC2</div>
           <p>No projects yet. Run a scan to register the first project.</p>
         </div>`
      : `<div class="projects-grid">${projects.map(renderProjectCard).join('')}</div>`
    }`;
}

function renderProjectCard(project) {
  const tags = (project.tags || []).filter(Boolean);
  const tagsHtml = tags.length
    ? '<div class="tags">' + tags.map(tag => '<span class="tag">' + escHtml(tag) + '</span>').join('') + '</div>'
    : '';

  const gateStatus = project.gate_status || '';
  const gateCls = cardClassForGateStatus(gateStatus);
  const gateBadge = gateStatus ? `<span class="badge ${badgeClassForGateStatus(gateStatus)}">${escHtml(gateStatus)}</span>` : '';

  return `<div class="project-card ${gateCls}" data-key="${escAttr(project.key)}">
    <div class="card-top">
      <span class="key">${escHtml(project.key)}</span>
      ${gateBadge}
    </div>
    <div class="name" title="${escAttr(project.name || project.key)}">${escHtml(project.name || project.key)}</div>
    ${tagsHtml}
    <div class="footer">Updated ${fmtDate(project.updated_at)}</div>
  </div>`;
}

export function showToast(msg, type = 'success') {
  const el = document.createElement('div');
  el.className = 'toast toast-' + type;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.classList.add('toast-show'), 10);
  setTimeout(() => {
    el.classList.remove('toast-show');
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

function bindMain() {
  document.getElementById('logoutBtn')?.addEventListener('click', logout);
  document.getElementById('apiDocsBtn')?.addEventListener('click', () => {
    state.view = 'api-docs';
    render();
  });
  document.getElementById('backgroundTasksBtn')?.addEventListener('click', async () => {
    resetProjectState();
    state.view = 'background-tasks';
    render();
    await loadBackgroundTasksData({ projectKey: '' });
  });
  document.getElementById('backgroundTasksBackBtn')?.addEventListener('click', () => loadProjects());
  document.getElementById('apiDocsBackBtn')?.addEventListener('click', () => loadProjects());
  document.getElementById('backBtn')?.addEventListener('click', () => loadProjects());
  document.querySelectorAll('.project-card').forEach(card => {
    card.addEventListener('click', () => loadProject(card.dataset.key, { project: card.dataset.key, tab: 'overview', branch: '', pullRequest: '' }));
  });
  if (state.view === 'project') {
    bindProjectViewControls();
  }
  if (state.view === 'background-tasks') {
    bindBackgroundTasksContent();
  }
}

export function logout() {
  clearStorage();
  history.replaceState({}, '', globalThis.location.pathname);
  replaceState(createInitialState());
  render();
}

function handleGlobalKeydown(event) {
  if (event.key === 'Escape' && state.selectedIssue) {
    closeIssueDetail();
  }
}

export async function init() {
  globalThis.addEventListener('popstate', async () => {
    if (!getToken()) return;
    const route = parseProjectRoute();
    if (route.project) {
      await loadProject(route.project, route);
      return;
    }
    await loadProjects();
  });

  const token = getToken();
  if (token) {
    state.user = loadUser();
    await loadUISettings();
    const route = parseProjectRoute();
    if (route.project) {
      await loadProject(route.project, route);
    } else {
      await loadProjects();
    }
  } else {
    render();
  }
}

let bootstrapped = false;

export function bootBrowserApp() {
  if (bootstrapped) return;
  bootstrapped = true;
  document.addEventListener('keydown', handleGlobalKeydown);
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
    return;
  }
  void init();
}