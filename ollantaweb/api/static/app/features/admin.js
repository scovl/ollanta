import { apiFetch } from '../core/api.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtDate, fmtNum } from '../core/utils.js';

let renderView = () => {};
let showToastMessage = () => {};

export function configureAdminFeature(options) {
  renderView = options.render;
  showToastMessage = options.showToast;
}

export async function loadGateData() {
  try {
    const data = await apiFetch('/quality-gates');
    state.gateData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.gateData = [];
  }
  renderView();
}

export function renderGateTab() {
  const gates = state.gateData;
  if (gates === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  if (!gates.length) return `<div class="empty-state" style="padding:40px 0"><p>No quality gates configured.</p></div>`;

  const rows = gates.map(gate => `
    <div class="gate-card">
      <div class="gate-header">
        <div>
          <span class="gate-name">${escHtml(gate.name)}</span>
          ${gate.is_default ? `<span class="badge badge-ok" style="font-size:11px;margin-left:8px">Default</span>` : ''}
        </div>
        <div class="gate-actions">
          <button class="btn-sm btn-outline assign-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Assign to project</button>
          <button class="btn-sm btn-ghost expand-gate-btn" data-gate-id="${gate.id}">Conditions \u25BE</button>
        </div>
      </div>
      <div class="gate-conditions hidden" id="gate-cond-${gate.id}">
        <div class="loading-inline">Loading\u2026</div>
      </div>
    </div>`).join('');

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Quality Gates</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Conditions that must pass for a project analysis to be considered successful.</p>
    <div class="gate-list">${rows}</div>
  </div>`;
}

export async function loadWebhooksData() {
  const project = state.currentProject;
  if (!project) return;
  try {
    const data = await apiFetch('/webhooks?project_key=' + encodeURIComponent(project.key));
    state.webhooksData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.webhooksData = [];
  }
  try {
    state.newCodePeriod = await apiFetch('/projects/' + encodeURIComponent(project.key) + '/new-code-period');
  } catch {
    state.newCodePeriod = null;
  }
  renderView();
}

export function renderWebhooksTab() {
  const webhooks = state.webhooksData;
  if (webhooks === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const ncp = state.newCodePeriod;
  const ncpStr = formatNewCodePeriod(ncp);

  const webhookRows = webhooks.length === 0
    ? `<div class="empty-state" style="padding:20px 0"><p>No webhooks configured.</p></div>`
    : webhooks.map(webhook => `
      <div class="webhook-row">
        <div class="webhook-info">
          <span class="webhook-name">${escHtml(webhook.name)}</span>
          <span class="webhook-url" title="${escAttr(webhook.url)}">${escHtml(webhook.url)}</span>
        </div>
        <div class="webhook-btns">
          <button class="btn-sm btn-outline test-wh-btn" data-wh-id="${webhook.id}">Test</button>
          <button class="btn-sm btn-danger del-wh-btn" data-wh-id="${webhook.id}">Delete</button>
        </div>
      </div>`).join('');

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Webhooks</p>
    <div class="webhook-list">${webhookRows}</div>
    <div class="create-form">
      <h4 style="font-size:14px;font-weight:600;margin-bottom:12px">Add webhook</h4>
      <div class="form-row">
        <input id="newWhName" class="filter-input" placeholder="Name" style="width:150px">
        <input id="newWhUrl" class="filter-input" placeholder="https://\u2026" style="flex:1;min-width:200px">
        <input id="newWhSecret" class="filter-input" placeholder="Secret (optional)" style="width:160px">
        <button class="btn btn-primary" id="addWhBtn" style="width:auto;padding:6px 18px;margin-top:0">Add</button>
      </div>
    </div>

    <p class="section-title" style="margin-top:32px">New Code Period</p>
    <div class="scan-info" style="grid-template-columns:1fr auto;gap:16px;align-items:center">
      <div>
        <div class="info-label">Current strategy</div>
        <div class="info-value" id="ncpDisplay">${ncpStr}</div>
      </div>
      <div class="form-row" style="justify-content:flex-end">
        <select id="ncpStrategy" class="filter-sel">
          <option value="auto"${(!ncp || ncp.strategy === 'auto') ? ' selected' : ''}>Auto</option>
          <option value="previous_version"${ncp?.strategy === 'previous_version' ? ' selected' : ''}>Previous version</option>
          <option value="number_of_days"${ncp?.strategy === 'number_of_days' ? ' selected' : ''}>Number of days</option>
          <option value="reference_branch"${ncp?.strategy === 'reference_branch' ? ' selected' : ''}>Reference branch</option>
        </select>
        <input id="ncpValue" class="filter-input" placeholder="Value (if needed)" style="width:140px" value="${escAttr(ncp?.value || '')}">
        <button class="btn btn-primary" id="saveNcpBtn" style="width:auto;padding:6px 18px;margin-top:0">Save</button>
      </div>
    </div>
  </div>`;
}

function formatNewCodePeriod(ncp) {
  if (!ncp) return 'auto (default)';
  const value = ncp.value ? ' \u2014 ' + escHtml(ncp.value) : '';
  return escHtml(ncp.strategy) + value;
}

export async function loadProfilesData() {
  try {
    const data = await apiFetch('/profiles');
    state.profilesData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.profilesData = [];
  }
  renderView();
}

export function renderProfilesTab() {
  const profiles = state.profilesData;
  if (profiles === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  if (!profiles.length) return `<div class="empty-state" style="padding:40px 0"><p>No quality profiles found.</p></div>`;

  const byLang = {};
  for (const profile of profiles) {
    if (!byLang[profile.language]) byLang[profile.language] = [];
    byLang[profile.language].push(profile);
  }

  const sections = Object.entries(byLang).map(([lang, profs]) => `
    <div class="profile-lang-section">
      <h4 class="profile-lang-title">${escHtml(lang)}</h4>
      <div class="profile-list">
        ${profs.map(profile => `
          <div class="profile-row">
            <div class="profile-info">
              <span class="profile-name">${escHtml(profile.name)}</span>
              ${profile.is_builtin ? `<span class="badge badge-ok" style="font-size:10px;margin-left:6px">Built-in</span>` : ''}
              ${profile.is_default ? `<span class="badge badge-warn" style="font-size:10px;margin-left:6px">Default</span>` : ''}
              <span style="color:var(--text-muted);font-size:12px;margin-left:8px">${profile.rule_count || 0} rules</span>
            </div>
            <button class="btn-sm btn-outline assign-profile-btn"
              data-profile-id="${profile.id}"
              data-profile-lang="${escAttr(profile.language)}"
              data-profile-name="${escAttr(profile.name)}">Assign to project</button>
          </div>`).join('')}
      </div>
    </div>`).join('');

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Quality Profiles</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Profiles define which rules are active for each language.</p>
    ${sections}
  </div>`;
}

const TASK_STATUSES = ['', 'queued', 'running', 'retrying', 'stale', 'failed', 'completed', 'cancelled'];
const TASK_TYPES = ['', 'scan', 'index', 'webhook'];

function currentTaskFilters() {
  const filters = state.backgroundTaskFilters || {};
  const project = state.currentProject;
  return {
    type: filters.type || '',
    status: filters.status || '',
    projectKey: filters.projectKey ?? (project?.key || ''),
    workerId: filters.workerId || '',
    scanId: filters.scanId || '',
    limit: filters.limit || 25,
    offset: filters.offset || 0,
  };
}

function buildTaskQuery(filters) {
  const params = new URLSearchParams();
  if (filters.type) params.set('type', filters.type);
  if (filters.status) params.set('status', filters.status);
  if (filters.projectKey) params.set('project_key', filters.projectKey);
  if (filters.workerId) params.set('worker_id', filters.workerId);
  if (filters.scanId) params.set('scan_id', filters.scanId);
  params.set('limit', String(filters.limit || 25));
  params.set('offset', String(filters.offset || 0));
  return params.toString();
}

export async function loadBackgroundTasksData(options = {}) {
  const filters = { ...currentTaskFilters(), ...options };
  state.backgroundTaskFilters = filters;
  state.loadingBackgroundTasks = true;
  state.backgroundTaskError = '';
  renderView();
  try {
    const query = buildTaskQuery(filters);
    const [list, summary] = await Promise.all([
      apiFetch('/admin/background-tasks?' + query),
      apiFetch('/admin/background-tasks/summary?' + query),
    ]);
    state.backgroundTasksData = list;
    state.backgroundTasksSummary = summary;
  } catch (err) {
    state.backgroundTasksData = { items: [], total: 0, limit: filters.limit, offset: filters.offset };
    state.backgroundTasksSummary = null;
    state.backgroundTaskError = err.message || 'Failed to load background tasks.';
  }
  state.loadingBackgroundTasks = false;
  renderView();
}

export async function loadBackgroundTaskDetail(taskId) {
  if (!taskId) return;
  state.loadingBackgroundTaskDetail = true;
  state.selectedBackgroundTask = null;
  state.backgroundTaskError = '';
  renderView();
  try {
    state.selectedBackgroundTask = await apiFetch('/admin/background-tasks/' + encodeURIComponent(taskId));
  } catch (err) {
    state.backgroundTaskError = err.message || 'Failed to load task details.';
  }
  state.loadingBackgroundTaskDetail = false;
  renderView();
}

export function renderAdminLinksTab() {
  return renderBackgroundTasksPage();
}

export function renderBackgroundTasksPage() {
  const filters = currentTaskFilters();
  const data = state.backgroundTasksData || { items: [], total: 0, limit: filters.limit, offset: filters.offset };
  const items = data.items || [];
  const total = data.total || 0;
  const summary = state.backgroundTasksSummary;
  const selected = state.selectedBackgroundTask;
  const canPrev = (filters.offset || 0) > 0;
  const canNext = (filters.offset || 0) + (filters.limit || 25) < total;
  const globalHeader = state.view === 'background-tasks'
    ? `<div class="page-header background-task-page-title">
        <div>
          <h2>Background Tasks</h2>
          <p>Operational queue for scan intake, indexing, and webhook delivery.</p>
        </div>
        <button class="back-btn" id="backgroundTasksBackBtn">Back to projects</button>
      </div>`
    : '';

  return `${globalHeader}<div class="tab-section background-tasks-page">
    <div class="background-task-header">
      <div>
        <p class="section-title">Queue activity</p>
        <p class="background-task-subtitle">Tasks are grouped by project context first; the technical job id is shown as secondary metadata.</p>
      </div>
      <button class="btn-sm btn-outline" id="refreshBackgroundTasksBtn">Refresh</button>
    </div>
    ${state.backgroundTaskError ? `<div class="error-msg background-task-error">${escHtml(state.backgroundTaskError)}</div>` : ''}
    ${renderTaskSummary(summary)}
    ${renderTaskFilters(filters)}
    <div class="background-task-layout">
      <section class="background-task-table-panel">
        ${state.loadingBackgroundTasks ? `<div class="loading-state"><div class="spinner"></div></div>` : renderTaskTable(items)}
        <div class="background-task-pagination">
          <span>${fmtNum(total)} tasks</span>
          <button class="btn-sm btn-outline" data-task-page="prev"${canPrev ? '' : ' disabled'}>Previous</button>
          <button class="btn-sm btn-outline" data-task-page="next"${canNext ? '' : ' disabled'}>Next</button>
        </div>
      </section>
      <aside class="background-task-detail-panel">
        ${state.loadingBackgroundTaskDetail ? `<div class="loading-state"><div class="spinner"></div></div>` : renderTaskDetail(selected)}
      </aside>
    </div>
  </div>`;
}

function renderTaskSummary(summary) {
  const cards = [
    ['Queued', summary?.queue_depth || 0, 'queued'],
    ['Running', summary?.running_count || 0, 'running'],
    ['Failed', summary?.failed_count || 0, 'failed'],
    ['Stale', summary?.stale_count || 0, 'stale'],
    ['Retrying', summary?.retry_count || 0, 'retrying'],
    ['Recent completions', summary?.recent_completion_count || 0, 'completed'],
  ];
  return `<div class="background-task-summary-grid">
    ${cards.map(([label, value, status]) => `<button class="background-task-summary-card" data-task-status-filter="${escAttr(status)}">
      <span>${escHtml(label)}</span>
      <strong>${fmtNum(value)}</strong>
    </button>`).join('')}
  </div>`;
}

function renderTaskFilters(filters) {
  return `<div class="background-task-filters">
    <label>Type<select class="filter-sel" id="taskTypeFilter">${TASK_TYPES.map(value => `<option value="${escAttr(value)}"${filters.type === value ? ' selected' : ''}>${value ? escHtml(value) : 'All'}</option>`).join('')}</select></label>
    <label>Status<select class="filter-sel" id="taskStatusFilter">${TASK_STATUSES.map(value => `<option value="${escAttr(value)}"${filters.status === value ? ' selected' : ''}>${value ? escHtml(value) : 'All'}</option>`).join('')}</select></label>
    <label>Project<input class="filter-input" id="taskProjectFilter" value="${escAttr(filters.projectKey || '')}" placeholder="project key"></label>
    <label>Scan<input class="filter-input" id="taskScanFilter" value="${escAttr(filters.scanId || '')}" placeholder="scan id"></label>
    <label>Worker<input class="filter-input" id="taskWorkerFilter" value="${escAttr(filters.workerId || '')}" placeholder="worker id"></label>
    <button class="btn-sm btn-primary" id="applyTaskFiltersBtn">Apply</button>
    <button class="btn-sm btn-outline" id="clearTaskFiltersBtn">Clear</button>
  </div>`;
}

function renderTaskTable(items) {
  if (!items.length) return `<div class="empty-state"><p>No background tasks match the current filters.</p></div>`;
  return `<table class="background-task-table">
    <thead><tr><th>Task</th><th>Status</th><th>Type</th><th>Submitted</th><th>Started</th><th>Duration</th><th>Worker</th><th>Scan</th><th>Error</th></tr></thead>
    <tbody>${items.map(task => `<tr class="background-task-row" data-task-id="${escAttr(task.id)}">
      <td>${renderTaskName(task)}</td>
      <td><span class="task-status task-status-${escAttr(task.status)}">${escHtml(task.status)}</span></td>
      <td>${renderTaskType(task)}</td>
      <td>${fmtDate(task.created_at)}</td>
      <td>${fmtDate(task.started_at)}</td>
      <td>${formatSeconds(task.duration_seconds)}</td>
      <td>${escHtml(task.worker_id || '-')}</td>
      <td>${escHtml(task.scan_id || '-')}</td>
      <td class="task-error-cell" title="${escAttr(task.last_error || '')}">${escHtml(task.last_error || '-')}</td>
    </tr>`).join('')}</tbody>
  </table>`;
}

function renderTaskName(task) {
  const primary = task.project_key || (task.project_id ? 'Project #' + task.project_id : task.type + ' task');
  const subtitle = [task.id, task.scan_id ? 'scan #' + task.scan_id : '', task.attempts ? 'attempts ' + task.attempts : ''].filter(Boolean).join(' · ');
  return `<div class="task-name-cell">
    <strong>${escHtml(primary)}</strong>
    <span>${escHtml(subtitle || task.id)}</span>
  </div>`;
}

function renderTaskType(task) {
  const labels = {
    scan: 'Scan processing',
    index: 'Search indexing',
    webhook: 'Webhook delivery',
  };
  return `<span class="task-type-chip task-type-${escAttr(task.type)}">${escHtml(labels[task.type] || task.type)}</span>`;
}

function renderTaskDetail(task) {
  if (!task) return `<div class="empty-state"><p>Select a task to inspect diagnostics and supported actions.</p></div>`;
  const details = task.details || {};
  const detailRows = Object.entries(details).map(([key, value]) => `<div class="info-row"><span>${escHtml(key)}</span><strong>${escHtml(String(value))}</strong></div>`).join('');
  const actions = task.supported_actions || [];
  const scannerParameters = renderScannerParameters(task.scanner_parameters);
  return `<div class="background-task-detail">
    <div class="task-detail-head">
      <div class="task-name-cell">
        <strong>${escHtml(task.project_key || task.id)}</strong>
        <span>${escHtml(task.id)}</span>
      </div>
      <span class="task-status task-status-${escAttr(task.status)}">${escHtml(task.status)}</span>
    </div>
    <div class="scan-info task-detail-grid">
      <div><div class="info-label">Type</div><div class="info-value">${escHtml(task.type)}</div></div>
      <div><div class="info-label">Internal status</div><div class="info-value">${escHtml(task.internal_status)}</div></div>
      <div><div class="info-label">Created</div><div class="info-value">${fmtDate(task.created_at)}</div></div>
      <div><div class="info-label">Started</div><div class="info-value">${fmtDate(task.started_at)}</div></div>
      <div><div class="info-label">Completed</div><div class="info-value">${fmtDate(task.completed_at)}</div></div>
      <div><div class="info-label">Next retry</div><div class="info-value">${fmtDate(task.next_attempt_at)}</div></div>
    </div>
    ${scannerParameters}
    ${task.last_error ? `<div class="task-diagnostic"><span>Last error</span><pre>${escHtml(task.last_error)}</pre></div>` : ''}
    <div class="task-detail-section"><h4>Details</h4>${detailRows || '<p class="muted">No type-specific details.</p>'}</div>
    <div class="task-detail-actions">
      ${actions.map(action => `<button class="btn-sm ${action === 'cancel' ? 'btn-danger' : 'btn-outline'}" data-task-action="${escAttr(action)}" data-task-id="${escAttr(task.id)}">${escHtml(action)}</button>`).join('') || '<span class="muted">No actions available for this state.</span>'}
    </div>
  </div>`;
}

function renderScannerParameters(parameters) {
  if (!parameters || !Object.keys(parameters).length) {
    return `<div class="task-detail-section scanner-params-section">
      <h4>Scanner parameters</h4>
      <p class="muted">Scanner parameters were not captured for this task. Run a new scan with the updated scanner to populate this section.</p>
    </div>`;
  }

  const options = parameters.scanner_options || {};
  const scope = parameters.analysis_scope || {};
  const tests = options.tests || parameters.test_signals || {};
  const rows = [
    ['Config file', options.config_path],
    ['Project directory', options.project_dir],
    ['Project key', options.project_key || scope.project_key],
    ['Sources', options.sources],
    ['Exclusions', options.exclusions],
    ['Format', options.format],
    ['Branch', options.branch || scope.branch],
    ['Commit', options.commit_sha || scope.commit_sha],
    ['Pull request', options.pull_request_key || scope.pull_request_key],
    ['Pull request branch', options.pull_request_branch],
    ['Pull request base', options.pull_request_base || scope.pull_request_base],
    ['Server URL', options.server],
    ['Wait for server job', options.server_wait],
    ['Wait timeout', options.server_wait_timeout],
    ['Wait poll', options.server_wait_poll],
    ['Local UI', options.local_ui],
    ['Local UI bind', options.bind],
    ['Local UI port', options.port],
    ['Debug', options.debug],
  ];

  return `<div class="task-detail-section scanner-params-section">
    <h4>Scanner parameters</h4>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
    ${renderTestParameters(tests)}
  </div>`;
}

function renderParameterRow(label, value) {
  return `<div class="info-row"><span>${escHtml(label)}</span><strong>${escHtml(formatParameterValue(value))}</strong></div>`;
}

function renderTestParameters(tests) {
  if (!tests || !Object.keys(tests).length) return '';
  const summary = tests.summary || {};
  const modules = Array.isArray(tests.modules) ? tests.modules : [];
  const rows = [
    ['Tests enabled', tests.enabled ?? summary.enabled],
    ['Mode', tests.mode],
    ['Discover modules', tests.discover],
    ['Run commands', tests.run],
    ['Command policy', tests.command_policy],
    ['Max report age', tests.max_report_age],
    ['Max depth', tests.max_depth],
    ['Max candidates', tests.max_candidates],
    ['Max report bytes', tests.max_report_bytes],
  ];
  const moduleCards = modules.length
    ? `<div class="scanner-test-modules">${modules.map(module => renderTestModuleParameters(module)).join('')}</div>`
    : '';
  return `<div class="scanner-test-params">
    <h5>Test signal parameters</h5>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
    ${moduleCards}
  </div>`;
}

function renderTestModuleParameters(module) {
  const rows = [
    ['Root', module.root],
    ['Language', module.language],
    ['Role', module.architecture_role],
    ['Policy', module.test_policy],
    ['Command', module.command],
    ['Artifact root', module.artifact_root],
    ['Report root', module.report_root],
    ['Coverage reports', module.coverage_reports],
    ['Test reports', module.test_reports],
    ['Mutation reports', module.mutation_reports],
    ['Native reports', module.native_reports],
    ['Owner', module.owner],
    ['Team', module.team],
    ['Integration required', module.integration_required],
  ];
  const title = module.name || module.root || 'test module';
  return `<div class="scanner-test-module">
    <strong>${escHtml(title)}</strong>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
  </div>`;
}

function formatParameterValue(value) {
  if (value === undefined || value === null || value === '') return '-';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '-';
  if (typeof value === 'boolean') return value ? 'yes' : 'no';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function formatSeconds(value) {
  if (value == null) return '-';
  const seconds = Number(value);
  if (seconds < 60) return seconds + 's';
  if (seconds < 3600) return Math.floor(seconds / 60) + 'm';
  if (seconds < 86400) return Math.floor(seconds / 3600) + 'h';
  return Math.floor(seconds / 86400) + 'd';
}

export function bindAdminTabContent() {
  const project = state.currentProject;
  bindBackgroundTasksContent();
  if (!project) return;

  document.querySelectorAll('.expand-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + id);
      if (!box) return;
      const hidden = box.classList.toggle('hidden');
      btn.textContent = hidden ? 'Conditions \u25BE' : 'Conditions \u25B4';
      if (hidden) {
        return;
      }

      if (box.innerHTML.includes('Loading')) {
        try {
          const gate = await apiFetch('/quality-gates/' + id);
          const conds = gate.conditions || [];
          if (!conds.length) {
            box.innerHTML = '<p style="color:var(--text-muted);padding:8px 0;font-size:13px">No conditions defined.</p>';
          } else {
            box.innerHTML = `<table class="conditions-table">
              <thead><tr><th>Metric</th><th>Operator</th><th>Threshold</th><th>New Code Only</th></tr></thead>
              <tbody>${conds.map(condition => `<tr>
                <td>${escHtml(condition.metric)}</td>
                <td>${escHtml(condition.operator)}</td>
                <td class="mono">${escHtml(String(condition.value))}</td>
                <td>${condition.on_new_code ? '\u2713' : ''}</td>
              </tr>`).join('')}</tbody>
            </table>`;
          }
        } catch {
          box.innerHTML = '<p style="color:var(--danger);font-size:13px">Failed to load conditions.</p>';
        }
      }
    });
  });

  document.querySelectorAll('.assign-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const name = btn.dataset.gateName;
      btn.disabled = true;
      try {
        await apiFetch('/projects/' + encodeURIComponent(project.key) + '/quality-gate', {
          method: 'POST',
          body: JSON.stringify({ gate_id: Number.parseInt(id, 10) }),
        });
        showToastMessage('Gate "' + name + '" assigned.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.getElementById('addWhBtn')?.addEventListener('click', async () => {
    const name = document.getElementById('newWhName')?.value.trim();
    const url = document.getElementById('newWhUrl')?.value.trim();
    const secret = document.getElementById('newWhSecret')?.value.trim();
    if (!name || !url) {
      showToastMessage('Name and URL are required.', 'error');
      return;
    }
    try {
      await apiFetch('/webhooks', {
        method: 'POST',
        body: JSON.stringify({ name, url, secret: secret || '', project_key: project.key }),
      });
      state.webhooksData = null;
      await loadWebhooksData();
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.test-wh-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/webhooks/' + btn.dataset.whId + '/test', { method: 'POST' });
        showToastMessage('Test delivery sent.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.del-wh-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Delete this webhook?')) return;
      btn.disabled = true;
      try {
        await apiFetch('/webhooks/' + btn.dataset.whId, { method: 'DELETE' });
        state.webhooksData = null;
        await loadWebhooksData();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.getElementById('saveNcpBtn')?.addEventListener('click', async () => {
    const strategy = document.getElementById('ncpStrategy')?.value;
    const value = document.getElementById('ncpValue')?.value.trim();
    try {
      await apiFetch('/projects/' + encodeURIComponent(project.key) + '/new-code-period', {
        method: 'PUT',
        body: JSON.stringify({ strategy, value: value || '' }),
      });
      state.newCodePeriod = { strategy, value };
      const display = document.getElementById('ncpDisplay');
      if (display) display.textContent = strategy + (value ? ' \u2014 ' + value : '');
      showToastMessage('New code period saved.');
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.assign-profile-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.profileId;
      const lang = btn.dataset.profileLang;
      const name = btn.dataset.profileName;
      btn.disabled = true;
      try {
        await apiFetch('/projects/' + encodeURIComponent(project.key) + '/profiles', {
          method: 'POST',
          body: JSON.stringify({ profile_id: Number.parseInt(id, 10), language: lang }),
        });
        showToastMessage('Profile "' + name + '" assigned.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });
}

export function bindBackgroundTasksContent() {
  document.getElementById('refreshBackgroundTasksBtn')?.addEventListener('click', () => loadBackgroundTasksData());
  document.getElementById('applyTaskFiltersBtn')?.addEventListener('click', () => {
    loadBackgroundTasksData({
      type: document.getElementById('taskTypeFilter')?.value || '',
      status: document.getElementById('taskStatusFilter')?.value || '',
      projectKey: document.getElementById('taskProjectFilter')?.value.trim() || '',
      scanId: document.getElementById('taskScanFilter')?.value.trim() || '',
      workerId: document.getElementById('taskWorkerFilter')?.value.trim() || '',
      offset: 0,
    });
  });
  document.getElementById('clearTaskFiltersBtn')?.addEventListener('click', () => loadBackgroundTasksData({ type: '', status: '', projectKey: '', scanId: '', workerId: '', offset: 0 }));
  document.querySelectorAll('[data-task-status-filter]').forEach(btn => {
    btn.addEventListener('click', () => loadBackgroundTasksData({ status: btn.dataset.taskStatusFilter || '', offset: 0 }));
  });
  document.querySelectorAll('[data-task-page]').forEach(btn => {
    btn.addEventListener('click', () => {
      const filters = currentTaskFilters();
      const delta = btn.dataset.taskPage === 'next' ? filters.limit : -filters.limit;
      loadBackgroundTasksData({ offset: Math.max(0, filters.offset + delta) });
    });
  });
  document.querySelectorAll('[data-task-id].background-task-row').forEach(row => {
    row.addEventListener('click', () => loadBackgroundTaskDetail(row.dataset.taskId));
  });
  document.querySelectorAll('[data-task-action]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const action = btn.dataset.taskAction;
      const id = btn.dataset.taskId;
      if (action === 'cancel' && !confirm('Cancel this queued background task?')) return;
      btn.disabled = true;
      try {
        state.selectedBackgroundTask = await apiFetch('/admin/background-tasks/' + encodeURIComponent(id) + '/' + action, { method: 'POST' });
        await loadBackgroundTasksData();
        showToastMessage('Task ' + action + ' accepted.');
      } catch (err) {
        showToastMessage(err.message || 'Task action failed.', 'error');
        btn.disabled = false;
      }
    });
  });
}