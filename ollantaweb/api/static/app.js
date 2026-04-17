'use strict';

const API = '/api/v1';

let state = {
  user: null,
  view: 'login',   // 'login' | 'projects' | 'project'
  projects: [],
  currentProject: null,
  currentScan: null,
  issues: [],
  issuesTotal: 0,
  issueOffset: 0,
  issueFilter: { severity: 'all', type: 'all', search: '' },
  loading: false,
  loadingIssues: false,
};

// ── Storage ───────────────────────────────────────────────────────────────────

function getToken()  { return localStorage.getItem('olt_token'); }
function saveToken(t) { localStorage.setItem('olt_token', t); }
function clearStorage() {
  localStorage.removeItem('olt_token');
  localStorage.removeItem('olt_user');
}
function saveUser(u) { localStorage.setItem('olt_user', JSON.stringify(u)); }
function loadUser()  {
  try { return JSON.parse(localStorage.getItem('olt_user') || 'null'); }
  catch { return null; }
}

// ── API helper ────────────────────────────────────────────────────────────────

async function apiFetch(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json' };
  const t = getToken();
  if (t) headers['Authorization'] = 'Bearer ' + t;
  if (opts.headers) Object.assign(headers, opts.headers);

  const res = await fetch(API + path, { ...opts, headers });

  if (res.status === 401) {
    logout();
    throw new Error('Session expired');
  }
  if (res.status === 204) return null;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || res.statusText);
  return body;
}

// ── Formatters ────────────────────────────────────────────────────────────────

function fmtDate(d) {
  if (!d) return '—';
  const date = new Date(d);
  const diff = Date.now() - date.getTime();
  if (diff < 60_000)     return 'just now';
  if (diff < 3_600_000)  return Math.floor(diff / 60_000) + 'm ago';
  if (diff < 86_400_000) return Math.floor(diff / 3_600_000) + 'h ago';
  if (diff < 604_800_000) return Math.floor(diff / 86_400_000) + 'd ago';
  return date.toLocaleDateString();
}

function fmtNum(n) {
  return (n == null ? 0 : Number(n)).toLocaleString();
}

// ── Render ────────────────────────────────────────────────────────────────────

function render() {
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
  const u = state.user || {};
  const name = u.name || u.login || 'User';
  return `<nav>
    <span class="logo">◈ Ollanta</span>
    <span class="user-info">${escHtml(name)}</span>
    <button class="logout-btn" id="logoutBtn">Sign out</button>
  </nav>`;
}

function renderContent() {
  if (state.view === 'projects') return renderDashboard();
  if (state.view === 'project')  return renderProjectDetail();
  return '';
}

// ── Login ─────────────────────────────────────────────────────────────────────

function renderLogin() {
  return `<div class="login-wrapper">
    <div class="login-card">
      <h1>◈ Ollanta</h1>
      <p class="subtitle">Static analysis platform</p>
      <div class="field">
        <label for="loginUser">Username</label>
        <input id="loginUser" type="text" placeholder="admin" autocomplete="username">
      </div>
      <div class="field">
        <label for="loginPass">Password</label>
        <input id="loginPass" type="password" placeholder="••••••••" autocomplete="current-password">
      </div>
      <button class="btn btn-primary" id="loginBtn">Sign in</button>
      <div id="loginError" class="error-msg"></div>
    </div>
  </div>`;
}

function bindLogin() {
  const btn   = document.getElementById('loginBtn');
  const errEl = document.getElementById('loginError');
  const userEl = document.getElementById('loginUser');
  const passEl = document.getElementById('loginPass');

  async function doLogin() {
    const login    = userEl.value.trim();
    const password = passEl.value;
    if (!login || !password) { errEl.textContent = 'Enter username and password.'; return; }

    btn.disabled = true;
    btn.textContent = 'Signing in…';
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
    } catch (e) {
      errEl.textContent = e.message || 'Login failed.';
      btn.disabled = false;
      btn.textContent = 'Sign in';
    }
  }

  btn.addEventListener('click', doLogin);
  passEl.addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
  userEl.addEventListener('keydown', e => { if (e.key === 'Enter') passEl.focus(); });
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

async function loadProjects() {
  state.view    = 'projects';
  state.loading = true;
  render();

  try {
    const data = await apiFetch('/projects?limit=100');
    state.projects = data.items || [];
  } catch {
    state.projects = [];
  }

  state.loading = false;
  render();
}

function renderDashboard() {
  if (state.loading) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  const ps = state.projects;
  const count = ps.length;

  return `
    <div class="page-header">
      <h2>Projects <span style="font-size:14px;color:var(--text-muted);font-weight:400">(${count})</span></h2>
      <p>All projects registered on this platform</p>
    </div>
    ${count === 0
      ? `<div class="empty-state">
           <div class="empty-icon">📂</div>
           <p>No projects yet. Run a scan to register the first project.</p>
         </div>`
      : `<div class="projects-grid">${ps.map(renderProjectCard).join('')}</div>`
    }`;
}

function renderProjectCard(p) {
  const tags = (p.tags || []).filter(Boolean);
  const tagsHtml = tags.length
    ? `<div class="tags">${tags.map(t => `<span class="tag">${escHtml(t)}</span>`).join('')}</div>`
    : '';

  return `<div class="project-card" data-key="${escAttr(p.key)}">
    <div class="key">${escHtml(p.key)}</div>
    <div class="name" title="${escAttr(p.name || p.key)}">${escHtml(p.name || p.key)}</div>
    ${tagsHtml}
    <div class="footer">Updated ${fmtDate(p.updated_at)}</div>
  </div>`;
}

// ── Project detail ────────────────────────────────────────────────────────────

const ISSUE_PAGE = 50;

async function loadProject(key) {
  state.view           = 'project';
  state.currentProject = null;
  state.currentScan    = null;
  state.issues         = [];
  state.issuesTotal    = 0;
  state.issueOffset    = 0;
  state.issueFilter    = { severity: 'all', type: 'all', search: '' };
  state.loading        = true;
  render();

  try {
    const [project, scan] = await Promise.all([
      apiFetch('/projects/' + encodeURIComponent(key)),
      apiFetch('/projects/' + encodeURIComponent(key) + '/scans/latest').catch(() => null),
    ]);
    state.currentProject = project;
    state.currentScan    = scan;
  } catch { /* ignore */ }

  state.loading = false;
  render();
  // Load issues after first render so metrics appear immediately
  await loadIssues();
}

async function loadIssues(append = false) {
  const p = state.currentProject;
  if (!p) return;
  if (!append) state.issueOffset = 0;

  state.loadingIssues = true;
  renderIssuesSection();

  const f      = state.issueFilter;
  const scanId  = state.currentScan?.id;
  // Prefer scan_id so the table matches the metrics panel (latest scan only)
  let qs = scanId
    ? `scan_id=${scanId}&limit=${ISSUE_PAGE}&offset=${state.issueOffset}`
    : `project_id=${p.id}&limit=${ISSUE_PAGE}&offset=${state.issueOffset}`;
  if (f.severity !== 'all') qs += `&severity=${encodeURIComponent(f.severity)}`;
  if (f.type     !== 'all') qs += `&type=${encodeURIComponent(f.type)}`;
  if (f.search)             qs += `&file=${encodeURIComponent(f.search)}`;

  try {
    const data = await apiFetch('/issues?' + qs);
    if (append) {
      state.issues = state.issues.concat(data.items || []);
    } else {
      state.issues = data.items || [];
    }
    state.issuesTotal = data.total || 0;
  } catch {
    if (!append) state.issues = [];
  }

  state.loadingIssues = false;
  renderIssuesSection();
}

function renderProjectDetail() {
  const backBtn = `<button class="back-btn" id="backBtn">← Projects</button>`;

  if (state.loading) {
    return backBtn + `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  const p = state.currentProject;
  if (!p) {
    return backBtn + `<div class="empty-state"><p>Project not found.</p></div>`;
  }

  const s = state.currentScan;
  const gateCls = !s ? '' : s.gate_status === 'OK' ? 'badge-ok' : s.gate_status === 'WARN' ? 'badge-warn' : 'badge-error';
  const gateBadge = s && s.gate_status ? `<span class="badge ${gateCls}">${escHtml(s.gate_status)}</span>` : '';

  const desc = [p.description, (p.tags || []).filter(Boolean).join(', ')].filter(Boolean).join(' · ');

  return `
    ${backBtn}
    <div class="detail-header">
      <h2>${escHtml(p.name || p.key)} ${gateBadge}</h2>
      <p>${escHtml(p.key)}${desc ? ' — ' + escHtml(desc) : ''}</p>
    </div>

    ${s ? renderScanMetrics(s) : `
      <div class="empty-state">
        <div class="empty-icon">🔬</div>
        <p>No scans yet for this project.<br>Run <code>ollanta</code> to submit a scan.</p>
      </div>`}

    <div id="issues-section"></div>`;
}

// ── Issues rendering ─────────────────────────────────────────────────────────

function renderIssuesSection() {
  const el = document.getElementById('issues-section');
  if (!el) return;
  el.innerHTML = buildIssuesHtml();
  bindIssueControls();
}

const SEV_ORDER  = ['blocker','critical','major','minor','info'];
const SEV_COLOR  = { blocker:'#e53e3e', critical:'#dd6b20', major:'#d69e2e', minor:'#3b82f6', info:'#6b7280' };
const SEV_BG     = { blocker:'rgba(229,62,62,.12)', critical:'rgba(221,107,32,.10)', major:'rgba(214,158,46,.09)', minor:'rgba(59,130,246,.09)', info:'rgba(107,114,128,.09)' };
const SEV_LABEL  = { blocker:'Blocker', critical:'Critical', major:'Major', minor:'Minor', info:'Info' };
const TYPE_ICON  = { bug:'🐛', code_smell:'🌿', vulnerability:'🔒' };

function sevCounts(issues) {
  const c = { blocker:0, critical:0, major:0, minor:0, info:0 };
  for (const i of issues) if (i.severity in c) c[i.severity]++;
  return c;
}

function buildIssuesHtml() {
  const issues = state.issues;
  const total  = state.issuesTotal;
  const f      = state.issueFilter;

  // ── Severity summary bar ──────────────────────────────────────────────
  const counts = sevCounts(issues);
  const chips  = SEV_ORDER.map(sev => {
    const n      = counts[sev];
    const active = f.severity === sev;
    return `<button class="sev-chip${active?' active':''}" data-sev="${sev}" style="--chip-color:${SEV_COLOR[sev]};--chip-bg:${SEV_BG[sev]}">
      <span class="chip-dot" style="background:${SEV_COLOR[sev]}"></span>
      ${SEV_LABEL[sev]}
      <span class="chip-count">${n}</span>
    </button>`;
  }).join('');

  const summaryBar = `<div class="sev-bar">${chips}</div>`;

  // ── Toolbar with type + search ────────────────────────────────────────
  const filtersHtml = `
    <div class="issues-toolbar">
      <span class="section-title" style="margin:0">Issues
        <span style="font-size:13px;font-weight:400;color:var(--text-muted)">&nbsp;${total.toLocaleString()} total</span>
      </span>
      <div class="issues-filters">
        <select id="filterType" class="filter-sel">
          <option value="all"${f.type==='all'?' selected':''}>All types</option>
          <option value="bug"${f.type==='bug'?' selected':''}>Bug</option>
          <option value="code_smell"${f.type==='code_smell'?' selected':''}>Code Smell</option>
          <option value="vulnerability"${f.type==='vulnerability'?' selected':''}>Vulnerability</option>
        </select>
        <input id="filterSearch" class="filter-input" type="text" placeholder="Search file or message…" value="${escAttr(f.search)}">
      </div>
    </div>`;

  if (state.loadingIssues && issues.length === 0) {
    return summaryBar + filtersHtml + `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  if (issues.length === 0) {
    return summaryBar + filtersHtml + `<div class="empty-state" style="padding:32px 0"><p>No issues match the current filters.</p></div>`;
  }

  const rows = issues.map(i => {
    const color = SEV_COLOR[i.severity] || '#6b7280';
    const bg    = SEV_BG[i.severity]   || 'transparent';
    const icon  = TYPE_ICON[i.type] || '?';
    const file  = (i.component_path || '').replace(/\\/g, '/').split('/').slice(-3).join('/');
    const loc   = i.end_line && i.end_line !== i.line ? `${i.line}–${i.end_line}` : `${i.line}`;
    return `<tr style="--row-sev-color:${color};--row-sev-bg:${bg}" class="sev-row">
      <td><span class="sev-badge" style="background:${color}">${escHtml(i.severity)}</span></td>
      <td>${icon} ${escHtml((i.type||'').replace('_',' ').toLowerCase())}</td>
      <td class="mono" style="font-size:11px">${escHtml(i.rule_key||'')}</td>
      <td class="file-cell" title="${escAttr(i.component_path||'')}"><span class="mono">${escHtml(file)}<span style="color:var(--text-muted)">:${loc}</span></span></td>
      <td>${escHtml(i.message||'')}</td>
    </tr>`;
  }).join('');

  const hasMore = issues.length < total;
  const moreBtn = hasMore
    ? `<div style="text-align:center;padding:16px">
        <button class="btn btn-primary" id="loadMoreBtn" style="width:auto;padding:8px 24px">
          ${state.loadingIssues ? 'Loading…' : `Load more (${total - issues.length} remaining)`}
        </button>
       </div>`
    : '';

  return summaryBar + filtersHtml + `
    <div class="issues-table-wrap">
      <table class="issues-table">
        <thead><tr>
          <th>Severity</th><th>Type</th><th>Rule</th><th>File</th><th>Message</th>
        </tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
    ${moreBtn}`;
}

function bindIssueControls() {
  // Severity chips — toggle: click active chip deselects, click inactive selects
  document.querySelectorAll('.sev-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      const sev = btn.dataset.sev;
      state.issueFilter.severity = state.issueFilter.severity === sev ? 'all' : sev;
      loadIssues();
    });
  });
  document.getElementById('filterType')?.addEventListener('change', e => {
    state.issueFilter.type = e.target.value;
    loadIssues();
  });
  let searchTimer;
  document.getElementById('filterSearch')?.addEventListener('input', e => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      state.issueFilter.search = e.target.value.trim();
      loadIssues();
    }, 300);
  });
  document.getElementById('loadMoreBtn')?.addEventListener('click', () => {
    state.issueOffset += ISSUE_PAGE;
    loadIssues(true);
  });
}

function renderScanMetrics(s) {
  const newIssuesCls = s.new_issues > 0 ? 'warning' : 'success';

  return `
    <div class="metrics-row">
      <div class="metric-card">
        <div class="label">Bugs</div>
        <div class="value ${s.total_bugs > 0 ? 'danger' : 'success'}">${fmtNum(s.total_bugs)}</div>
      </div>
      <div class="metric-card">
        <div class="label">Vulnerabilities</div>
        <div class="value ${s.total_vulnerabilities > 0 ? 'warning' : 'success'}">${fmtNum(s.total_vulnerabilities)}</div>
      </div>
      <div class="metric-card">
        <div class="label">Code Smells</div>
        <div class="value muted">${fmtNum(s.total_code_smells)}</div>
      </div>
      <div class="metric-card">
        <div class="label">Total Issues</div>
        <div class="value info">${fmtNum(s.total_issues)}</div>
      </div>
      <div class="metric-card">
        <div class="label">New Issues</div>
        <div class="value ${newIssuesCls}">${fmtNum(s.new_issues)}</div>
      </div>
      <div class="metric-card">
        <div class="label">Files</div>
        <div class="value">${fmtNum(s.total_files)}</div>
      </div>
      <div class="metric-card">
        <div class="label">Lines</div>
        <div class="value">${fmtNum(s.total_lines)}</div>
      </div>
      <div class="metric-card">
        <div class="label">NCLOC</div>
        <div class="value">${fmtNum(s.total_ncloc)}</div>
      </div>
    </div>

    <p class="section-title">Latest scan</p>
    <div class="scan-info">
      <div>
        <div class="info-label">Version</div>
        <div class="info-value">${escHtml(s.version || '—')}</div>
      </div>
      <div>
        <div class="info-label">Branch</div>
        <div class="info-value">${escHtml(s.branch || '—')}</div>
      </div>
      <div>
        <div class="info-label">Commit</div>
        <div class="info-value" style="font-family:monospace;font-size:12px">${s.commit_sha ? escHtml(s.commit_sha.slice(0,8)) : '—'}</div>
      </div>
      <div>
        <div class="info-label">Status</div>
        <div class="info-value">${escHtml(s.status || '—')}</div>
      </div>
      <div>
        <div class="info-label">Gate</div>
        <div class="info-value">${escHtml(s.gate_status || '—')}</div>
      </div>
      <div>
        <div class="info-label">Analysis date</div>
        <div class="info-value">${s.analysis_date ? new Date(s.analysis_date).toLocaleString() : '—'}</div>
      </div>
      <div>
        <div class="info-label">Elapsed</div>
        <div class="info-value">${s.elapsed_ms ? (s.elapsed_ms / 1000).toFixed(1) + 's' : '—'}</div>
      </div>
    </div>`;
}

// ── Event binding ─────────────────────────────────────────────────────────────

function bindMain() {
  document.getElementById('logoutBtn')?.addEventListener('click', logout);
  document.getElementById('backBtn')?.addEventListener('click', () => loadProjects());
  document.querySelectorAll('.project-card').forEach(card => {
    card.addEventListener('click', () => loadProject(card.dataset.key));
  });
  if (state.view === 'project') bindIssueControls();
}

function logout() {
  clearStorage();
  state = { user: null, view: 'login', projects: [], currentProject: null, currentScan: null,
            issues: [], issuesTotal: 0, issueOffset: 0, issueFilter: { severity: 'all', type: 'all', search: '' },
            loading: false, loadingIssues: false };
  render();
}

// ── Security helpers ──────────────────────────────────────────────────────────

function escHtml(s) {
  if (s == null) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escAttr(s) { return escHtml(s); }

// ── Boot ──────────────────────────────────────────────────────────────────────

async function init() {
  const t = getToken();
  if (t) {
    state.user = loadUser();
    await loadProjects();
  } else {
    render();
  }
}

document.addEventListener('DOMContentLoaded', init);
