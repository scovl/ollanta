'use strict';

import { apiFetch } from './core/api.js';
import {
  activeScopeSubtitle,
  activeScopeTitle,
  buildScopedPath,
  normalizeScope,
  parseProjectRoute,
  resolvedBranchName,
  syncProjectUrl,
} from './core/scope.js';
import { resetProjectState, state } from './core/state.js';
import { badgeClassForGateStatus, escAttr, escHtml, fmtDate, fmtK, fmtNum } from './core/utils.js';
import { bindAdminTabContent, loadAIProvidersData, loadCustomRulesData, loadGateData, loadProfilesData, loadWebhooksData, renderAIProvidersTab, renderCustomRulesTab, renderGateTab, renderProfilesTab, renderWebhooksTab } from './features/admin.js';
import { bindActivityTabContent, loadActivityData, renderActivityTab } from './features/activity.js';
import { loadCodeFileData, loadCodeTreeData, renderCoverageTab } from './features/code.js';
import { loadIssues, openIssueDetail, renderIssuesSection } from './features/issues.js';
import { renderOverviewTab } from './features/overview.js';
import { loadProjectInfoData } from './features/project-information.js';

let renderView = () => {};

export function configureProjectFlowFeature(options) {
  renderView = options.render;
}

function normalizeProjectTab(tab) {
  if (tab === 'admin' || tab === 'information') return 'overview';
  if (tab === 'branches' || tab === 'pull-requests') return 'scopes';
  return tab === 'code' ? 'coverage' : tab;
}

function scopesTabHasContent() {
  const branches = (state.branchesData || []).filter(item => item?.name);
  const prs = state.pullRequestsData || [];
  return branches.length > 1 || prs.length > 0;
}

function renderProjectTabs(activeTab, issueCount) {
  const tabs = ['overview', 'issues', 'coverage', 'activity', 'scopes', 'gate', 'profiles', 'custom-rules', 'ai-providers', 'webhooks'];
  const labels = {
    overview: 'Overview',
    issues: 'Issues',
    activity: 'Activity',
    scopes: 'Scopes',
    coverage: 'Coverage',
    gate: 'Quality Gate',
    webhooks: 'Webhooks',
    profiles: 'Profiles',
    'custom-rules': 'Rule Studio',
    'ai-providers': 'AI Providers',
  };
  const showScopes = scopesTabHasContent();
  return `<div class="proj-tabs">${tabs.map(tab => {
    if (tab === 'scopes' && !showScopes) return '';
    const badge = tab === 'issues' && issueCount !== '' ? `<span class="tab-badge">${fmtK(issueCount)}</span>` : '';
    const active = tab === activeTab ? ' active' : '';
    return `<button class="tab-btn${active}" data-tab="${tab}">${labels[tab]}${badge}</button>`;
  }).join('')}</div>`;
}

function renderProjectTabContent(tab) {
  if (tab === 'issues') return `<div id="issues-section"></div>`;
  if (tab === 'activity') return renderActivityTab();
  if (tab === 'scopes') return renderScopesTab();
  if (tab === 'coverage') return renderCoverageTab();
  if (tab === 'gate') return renderGateTab();
  if (tab === 'webhooks') return renderWebhooksTab();
  if (tab === 'profiles') return renderProfilesTab();
  if (tab === 'custom-rules') return renderCustomRulesTab();
  if (tab === 'ai-providers') return renderAIProvidersTab();
  return renderOverviewTab();
}

export async function loadProject(key, route) {
  const nextRoute = route || parseProjectRoute();
  state.view = 'project';
  resetProjectState();
  state.currentProject = { key };
  state.projectTab = normalizeProjectTab(nextRoute.tab || 'overview');
  state.scope = normalizeScope({
    type: nextRoute.pullRequest ? 'pull_request' : 'branch',
    branch: nextRoute.branch,
    pullRequestKey: nextRoute.pullRequest,
  });
  state.loading = true;
  renderView();

  try {
    state.currentProject = await apiFetch('/projects/' + encodeURIComponent(key));
    await refreshProjectScope({ reloadCatalogs: true, replaceUrl: true });
  } catch {
    state.currentProject = null;
  }

  state.loading = false;
  renderView();
  await ensureProjectTabLoaded(state.projectTab);
}

async function refreshProjectScope(options) {
  const settings = { reloadCatalogs: true, replaceUrl: false, ...options };
  const key = state.currentProject?.key;
  if (!key) return;

  const requests = settings.reloadCatalogs
    ? [
        apiFetch(buildScopedPath('/projects/' + encodeURIComponent(key) + '/overview', state.scope)).catch(() => null),
        apiFetch('/projects/' + encodeURIComponent(key) + '/branches').catch(() => ({ items: [] })),
        apiFetch('/projects/' + encodeURIComponent(key) + '/pull-requests').catch(() => ({ items: [] })),
      ]
    : [apiFetch(buildScopedPath('/projects/' + encodeURIComponent(key) + '/overview', state.scope)).catch(() => null)];

  const results = await Promise.all(requests);
  const overview = results[0] || null;
  state.overviewData = overview;
  state.currentScan = overview?.last_scan || null;
  if (overview?.scope) {
    state.scope = normalizeScope(overview.scope);
  }
  if (settings.reloadCatalogs) {
    state.branchesData = results[1]?.items || [];
    state.pullRequestsData = results[2]?.items || [];
  }

  if (settings.replaceUrl) {
    syncProjectUrl(true);
  }
}

async function loadBranchesData() {
  const key = state.currentProject?.key;
  if (!key) return;
  try {
    const data = await apiFetch('/projects/' + encodeURIComponent(key) + '/branches');
    state.branchesData = data.items || [];
    if (data.default_branch && !state.scope.defaultBranch) {
      state.scope.defaultBranch = data.default_branch;
    }
  } catch {
    state.branchesData = [];
  }
  renderView();
}

async function loadPullRequestsData() {
  const key = state.currentProject?.key;
  if (!key) return;
  try {
    const data = await apiFetch('/projects/' + encodeURIComponent(key) + '/pull-requests');
    state.pullRequestsData = data.items || [];
  } catch {
    state.pullRequestsData = [];
  }
  renderView();
}

async function loadScopesData() {
  const key = state.currentProject?.key;
  if (!key) return;
  const [branches, pullRequests] = await Promise.all([
    apiFetch('/projects/' + encodeURIComponent(key) + '/branches').catch(() => ({ items: [] })),
    apiFetch('/projects/' + encodeURIComponent(key) + '/pull-requests').catch(() => ({ items: [] })),
  ]);
  state.branchesData = branches.items || [];
  state.pullRequestsData = pullRequests.items || [];
  if (branches.default_branch && !state.scope.defaultBranch) {
    state.scope.defaultBranch = branches.default_branch;
  }
  renderView();
}

const projectTabLoaders = {
  overview: {
    shouldLoad: () => state.projectInfoData === null,
    load: loadProjectInfoData,
  },
  issues: {
    shouldLoad: () => state.issues.length === 0 && !state.loadingIssues,
    load: loadIssues,
  },
  activity: {
    shouldLoad: () => state.activityData === null,
    load: loadActivityData,
  },
  scopes: {
    shouldLoad: () => state.branchesData === null || state.pullRequestsData === null,
    load: loadScopesData,
  },
  coverage: {
    shouldLoad: () => state.codeTreeData === null,
    load: loadCodeTreeData,
  },
  gate: {
    shouldLoad: () => state.gateData === null,
    load: loadGateData,
  },
  webhooks: {
    shouldLoad: () => state.webhooksData === null,
    load: loadWebhooksData,
  },
  profiles: {
    shouldLoad: () => state.profilesData === null,
    load: loadProfilesData,
  },
  'custom-rules': {
    shouldLoad: () => state.customRulesData === null || state.customRuleEngines === null || state.profilesData === null,
    load: loadCustomRulesData,
  },
  'ai-providers': {
    shouldLoad: () => state.customRuleAIProviders === null,
    load: loadAIProvidersData,
  },
};

async function ensureProjectTabLoaded(tab) {
  const loader = projectTabLoaders[tab];
  if (loader?.shouldLoad()) {
    await loader.load();
  }
}

export async function changeScope(scope) {
  state.projectTab = normalizeProjectTab(state.projectTab);
  state.scope = normalizeScope(scope);
  state.currentScan = null;
  state.overviewData = null;
  state.activityData = null;
  state.projectInfoData = null;
  state.codeTreeData = null;
  state.codeFileData = null;
  state.codeSelectedPath = '';
  state.issues = [];
  state.issuesTotal = 0;
  state.issueOffset = 0;
  state.loading = true;
  syncProjectUrl(false);
  renderView();

  await refreshProjectScope({ reloadCatalogs: true, replaceUrl: true });

  state.loading = false;
  renderView();
  await ensureProjectTabLoaded(state.projectTab);
}

export function renderProjectDetail() {
  const backBtn = `<button class="back-btn" id="backBtn">\u2190 Projects</button>`;

  if (state.loading) {
    return backBtn + `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  const project = state.currentProject;
  if (!project) {
    return backBtn + `<div class="empty-state"><p>Project not found.</p></div>`;
  }

  const gateStatus = state.currentScan?.gate_status || '';
  const gateBadge = gateStatus ? `<span class="badge ${badgeClassForGateStatus(gateStatus)}">${escHtml(gateStatus)}</span>` : '';
  const desc = [project.description, (project.tags || []).filter(Boolean).join(', ')].filter(Boolean).join(' \u00B7 ');
  const issueCount = state.overviewData?.last_scan?.total_issues ?? '';

  return `
    ${backBtn}
    <div class="detail-header">
      <h2>${escHtml(project.name || project.key)} ${gateBadge}</h2>
      <p>${escHtml(project.key)}${desc ? ' \u2014 ' + escHtml(desc) : ''}</p>
    </div>
    ${renderScopeToolbar()}
    ${renderProjectTabs(state.projectTab, issueCount)}
    ${renderProjectTabContent(state.projectTab)}`;
}

export function renderScopeToolbar() {
  const scope = normalizeScope(state.scope);
  const branches = (state.branchesData || []).filter(item => item?.name);
  const pullRequests = state.pullRequestsData || [];
  const branchOptions = branches.length
    ? branches.map(item => `<option value="${escAttr(item.name)}">${escHtml(item.name)}</option>`).join('')
    : '<option value="">No detected branch</option>';
  const prOptions = [`<option value="">No pull request</option>`].concat(
    pullRequests.map(item => `<option value="${escAttr(item.key)}">#${escHtml(item.key)} - ${escHtml(item.branch || 'unknown')} -> ${escHtml(item.base_branch || 'unknown')}</option>`)
  ).join('');

  return `<div class="scope-toolbar">
    <div class="scope-summary-card">
      <span class="scope-kicker">Active scope</span>
      <div class="scope-title">${activeScopeTitle(scope)}</div>
      <div class="scope-subtitle">${activeScopeSubtitle(scope)}</div>
      <div class="scope-meta">
        <span>Default branch: ${escHtml(scope.defaultBranch || '\u2014')}</span>
        <span>Latest analysis: ${state.currentScan?.analysis_date ? fmtDate(state.currentScan.analysis_date) : 'none yet'}</span>
      </div>
    </div>
    <div class="scope-controls">
      <label class="scope-field">
        <span>Branch</span>
        <select id="scopeBranchSelect" class="filter-sel"${branches.length ? '' : ' disabled'}>
          ${branchOptions}
        </select>
      </label>
      <label class="scope-field">
        <span>Pull Request</span>
        <select id="scopePullRequestSelect" class="filter-sel">
          ${prOptions}
        </select>
      </label>
    </div>
  </div>`;
}

function renderBranchScanMeta(scan) {
  if (!scan) {
    return `<div class="scope-card-meta"><span>No successful scans yet.</span></div>`;
  }
  return `<div class="scope-card-meta">
    <span>${fmtDate(scan.analysis_date)}</span>
    <span>${fmtNum(scan.total_issues || 0)} issues</span>
    <span>${escHtml(scan.gate_status || 'NO GATE')}</span>
  </div>`;
}

function renderBranchScopeGrid(branchItems) {
  const items = branchItems.filter(item => item?.name);
  if (items.length === 0) {
    return `<div class="empty-state compact"><p>No named branches with analyses yet.</p></div>`;
  }

  const activeBranch = resolvedBranchName(state.scope);
  return `<div class="scope-grid">
    ${items.map(item => {
      const selected = state.scope.type !== 'pull_request' && item.name === activeBranch;
      return `<button class="scope-card${selected ? ' active' : ''}" data-branch-card="${escAttr(item.name)}">
        <div class="scope-card-head">
          <div>
            <div class="scope-card-title">${escHtml(item.name)}</div>
            <div class="scope-card-subtitle">${item.is_default ? 'Default branch' : 'Branch scope'}</div>
          </div>
          ${item.is_default ? '<span class="badge badge-ok">default</span>' : ''}
        </div>
        ${renderBranchScanMeta(item.latest_scan)}
      </button>`;
    }).join('')}
  </div>`;
}

function renderPullRequestScopeGrid(items) {
  if (items.length === 0) {
    return `<div class="empty-state compact"><p>No pull request analyses yet.</p></div>`;
  }

  return `<div class="scope-grid">
    ${items.map(item => {
      const selected = state.scope.type === 'pull_request' && item.key === state.scope.pullRequestKey;
      const badgeClass = badgeClassForGateStatus(item.latest_scan?.gate_status || 'ERROR');
      const badgeLabel = item.latest_scan?.gate_status || 'pending';
      return `<button class="scope-card${selected ? ' active' : ''}" data-pr-card="${escAttr(item.key)}" data-pr-branch="${escAttr(item.branch || '')}" data-pr-base="${escAttr(item.base_branch || '')}">
        <div class="scope-card-head">
          <div>
            <div class="scope-card-title">Pull Request #${escHtml(item.key)}</div>
            <div class="scope-card-subtitle">${escHtml(item.branch || 'unknown')} \u2192 ${escHtml(item.base_branch || 'unknown')}</div>
          </div>
          <span class="badge ${badgeClass}">${escHtml(badgeLabel)}</span>
        </div>
        ${renderBranchScanMeta(item.latest_scan)}
      </button>`;
    }).join('')}
  </div>`;
}

function renderScopesTab() {
  if (state.branchesData === null || state.pullRequestsData === null) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }
  if (!scopesTabHasContent()) {
    state.projectTab = 'overview';
    return renderOverviewTab();
  }

  const branches = (state.branchesData || []).filter(item => item?.name);
  const prs = state.pullRequestsData || [];

  return `<div class="scope-catalog">
    <section class="scope-catalog-section panel-surface">
      <p class="section-title">Branches <span class="section-count">${branches.length}</span></p>
      ${renderBranchScopeGrid(branches)}
    </section>
    <section class="scope-catalog-section panel-surface">
      <p class="section-title">Pull Requests <span class="section-count">${prs.length}</span></p>
      ${renderPullRequestScopeGrid(prs)}
    </section>
  </div>`;
}

export async function switchTab(tab) {
  state.projectTab = normalizeProjectTab(tab);
  syncProjectUrl(false);
  renderView();
  await ensureProjectTabLoaded(state.projectTab);
  if (state.projectTab === 'issues' && state.issues.length > 0) renderIssuesSection();
}

function bindTabContent() {
  const project = state.currentProject;
  if (!project) return;

  const branchSelect = document.getElementById('scopeBranchSelect');
  if (branchSelect) {
    branchSelect.value = state.scope.type === 'branch'
      ? resolvedBranchName(state.scope)
      : resolvedBranchName({ type: 'branch', branch: state.scope.defaultBranch, defaultBranch: state.scope.defaultBranch });
    branchSelect.addEventListener('change', () => {
      changeScope({ type: 'branch', branch: branchSelect.value, defaultBranch: state.scope.defaultBranch });
    });
  }

  const prSelect = document.getElementById('scopePullRequestSelect');
  if (prSelect) {
    prSelect.value = state.scope.type === 'pull_request' ? state.scope.pullRequestKey : '';
    prSelect.addEventListener('change', () => {
      if (!prSelect.value) {
        changeScope({ type: 'branch', branch: branchSelect?.value || resolvedBranchName(state.scope), defaultBranch: state.scope.defaultBranch });
        return;
      }
      const selected = (state.pullRequestsData || []).find(item => item.key === prSelect.value);
      changeScope({
        type: 'pull_request',
        pullRequestKey: prSelect.value,
        branch: selected?.branch || '',
        pullRequestBase: selected?.base_branch || '',
        defaultBranch: state.scope.defaultBranch,
      });
    });
  }

  document.querySelectorAll('[data-branch-card]').forEach(btn => {
    btn.addEventListener('click', () => {
      changeScope({ type: 'branch', branch: btn.dataset.branchCard, defaultBranch: state.scope.defaultBranch });
    });
  });

  document.querySelectorAll('[data-pr-card]').forEach(btn => {
    btn.addEventListener('click', () => {
      changeScope({
        type: 'pull_request',
        pullRequestKey: btn.dataset.prCard,
        branch: btn.dataset.prBranch || '',
        pullRequestBase: btn.dataset.prBase || '',
        defaultBranch: state.scope.defaultBranch,
      });
    });
  });

  document.querySelectorAll('[data-coverage-source-path]').forEach(btn => {
    btn.addEventListener('click', () => {
      loadCodeFileData(btn.dataset.coverageSourcePath);
    });
  });

  document.querySelectorAll('[data-code-issue]').forEach(btn => {
    btn.addEventListener('click', () => {
      const issue = state.codeFileData?.issues?.[Number(btn.dataset.codeIssue)];
      if (issue) openIssueDetail(issue);
    });
  });

  bindAdminTabContent();
  bindActivityTabContent();
}

function resetIssueFilters() {
  state.issueFilter.quality = 'all';
  state.issueFilter.severity = 'all';
  state.issueFilter.type = 'all';
  state.issueFilter.status = 'all';
  state.issueFilter.trackingState = 'all';
  state.issueFilter.language = 'all';
  state.issueFilter.rule = 'all';
  state.issueFilter.tag = 'all';
  state.issueFilter.securityCategory = 'all';
  state.issueFilter.directory = 'all';
  state.issueFilter.file = 'all';
  state.issueFilter.search = '';
  state.issueFilter.scanId = '';
  state.issues = [];
  state.issuesTotal = 0;
  state.issueOffset = 0;
}

function openIssuesFromOverview(updates) {
  resetIssueFilters();
  Object.assign(state.issueFilter, updates);
  switchTab('issues');
}

function handleOverviewAction(action) {
  if (action === 'coverage') {
    switchTab('coverage');
    return;
  }
  if (action === 'new-issues') {
    openIssuesFromOverview({ trackingState: 'new' });
    return;
  }
  if (action === 'closed-issues') {
    openIssuesFromOverview({ status: 'closed' });
  }
}

export function bindProjectViewControls() {
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  document.querySelectorAll('.metric-card.clickable, .metric-signal.clickable').forEach(btn => {
    btn.addEventListener('click', () => {
      const action = btn.dataset.overviewAction;
      if (action) {
        handleOverviewAction(action);
        return;
      }
      const type = btn.dataset.mcType;
      if (type) {
        openIssuesFromOverview({ type: type === 'all' ? 'all' : type });
      }
    });
  });

  document.querySelectorAll('.hotspot-row').forEach(row => {
    row.addEventListener('click', () => {
      if (row.dataset.overviewIssueIndex != null) {
        const issue = state.overviewData?.summary?.must_fix_now?.[Number(row.dataset.overviewIssueIndex)];
        if (issue) openIssueDetail(issue);
        return;
      }
      const file = row.dataset.file;
      if (file) {
        openIssuesFromOverview({ search: file });
      }
    });
  });

  document.querySelectorAll('[data-coverage-path]').forEach(row => {
    row.addEventListener('click', () => {
      const file = row.dataset.coveragePath;
      if (file) {
        state.codeSelectedPath = file;
        switchTab('coverage');
      }
    });
  });

  bindTabContent();
}
