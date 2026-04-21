import { apiFetch } from '../core/api.js';
import { buildScopeQuery } from '../core/scope.js';
import {
  ISSUE_PAGE,
  SEV_BG,
  SEV_COLOR,
  SEV_LABEL,
  SEV_ORDER,
  TYPE_ICON,
  TYPE_LABEL,
  state,
} from '../core/state.js';
import { escAttr, escHtml } from '../core/utils.js';

let showToastMessage = () => {};

export function configureIssuesFeature(options) {
  showToastMessage = options.showToast;
}

export async function loadIssues(append) {
  const project = state.currentProject;
  if (!project) return;
  if (!append) state.issueOffset = 0;

  state.loadingIssues = true;
  renderIssuesSection();

  const filter = state.issueFilter;
  const qs = new URLSearchParams({
    project_key: project.key,
    limit: String(ISSUE_PAGE),
    offset: String(state.issueOffset),
  });
  buildScopeQuery(state.scope).forEach((value, key) => qs.set(key, value));
  if (filter.severity !== 'all') qs.set('severity', filter.severity);
  if (filter.type !== 'all') qs.set('type', filter.type);
  if (filter.status !== 'all') qs.set('status', filter.status);
  if (filter.search) qs.set('file', filter.search);

  try {
    const data = await apiFetch('/issues?' + qs.toString());
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

export function renderIssuesSection() {
  const el = document.getElementById('issues-section');
  if (!el) return;
  el.innerHTML = buildIssuesHtml();
  bindIssueControls();
}

function sevCounts(issues) {
  const counts = { blocker: 0, critical: 0, major: 0, minor: 0, info: 0 };
  for (const issue of issues) {
    if (issue.severity in counts) counts[issue.severity] += 1;
  }
  return counts;
}

function buildIssuesHtml() {
  const issues = state.issues;
  const total = state.issuesTotal;
  const filter = state.issueFilter;

  const counts = sevCounts(issues);
  const chips = SEV_ORDER.map(sev => {
    const count = counts[sev];
    const active = filter.severity === sev;
    return `<button class="sev-chip${active ? ' active' : ''}" data-sev="${sev}" style="--chip-color:${SEV_COLOR[sev]};--chip-bg:${SEV_BG[sev]}">
      <span class="chip-dot" style="background:${SEV_COLOR[sev]}"></span>
      ${SEV_LABEL[sev]}
      <span class="chip-count">${count}</span>
    </button>`;
  }).join('');

  const summaryBar = `<div class="sev-bar">${chips}</div>`;

  const filtersHtml = `
    <div class="issues-toolbar">
      <span class="section-title" style="margin:0">Issues
        <span style="font-size:13px;font-weight:400;color:var(--text-muted)">&nbsp;${total.toLocaleString()} total</span>
      </span>
      <div class="issues-filters">
        <select id="filterType" class="filter-sel">
          <option value="all"${filter.type === 'all' ? ' selected' : ''}>All types</option>
          <option value="bug"${filter.type === 'bug' ? ' selected' : ''}>Bug</option>
          <option value="code_smell"${filter.type === 'code_smell' ? ' selected' : ''}>Code Smell</option>
          <option value="vulnerability"${filter.type === 'vulnerability' ? ' selected' : ''}>Vulnerability</option>
        </select>
        <select id="filterStatus" class="filter-sel">
          <option value="all"${filter.status === 'all' ? ' selected' : ''}>All statuses</option>
          <option value="open"${filter.status === 'open' ? ' selected' : ''}>Open</option>
          <option value="closed"${filter.status === 'closed' ? ' selected' : ''}>Closed</option>
        </select>
        <input id="filterSearch" class="filter-input" type="text" placeholder="Search file or message\u2026" value="${escAttr(filter.search)}">
      </div>
    </div>`;

  if (state.loadingIssues && issues.length === 0) {
    return summaryBar + filtersHtml + `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  if (issues.length === 0) {
    return summaryBar + filtersHtml + `<div class="empty-state" style="padding:32px 0"><p>No issues match the current filters.</p></div>`;
  }

  const rows = issues.map((issue, idx) => {
    const color = SEV_COLOR[issue.severity] || '#64748b';
    const bg = SEV_BG[issue.severity] || 'transparent';
    const icon = TYPE_ICON[issue.type] || '?';
    const file = (issue.component_path || '').replace(/\\/g, '/').split('/').slice(-3).join('/');
    const loc = issue.end_line && issue.end_line !== issue.line ? `${issue.line}\u2013${issue.end_line}` : `${issue.line}`;
    const status = issue.status || 'open';
    const isClosed = status === 'closed';
    let actionBtns = '';
    if (!isClosed) {
      actionBtns = `
        <button class="itbtn fp-btn" data-id="${issue.id}" data-res="false_positive" title="False positive">FP</button>
        <button class="itbtn wf-btn" data-id="${issue.id}" data-res="wont_fix" title="Won\u2019t fix">WF</button>
        <button class="itbtn ok-btn" data-id="${issue.id}" data-res="fixed" title="Mark as fixed">\u2713</button>`;
    } else {
      actionBtns = `<button class="itbtn re-btn" data-id="${issue.id}" data-res="" title="Reopen">\u21A9</button>`;
    }
    return `<tr style="--row-sev-color:${color};--row-sev-bg:${bg}" class="sev-row${isClosed ? ' row-closed' : ''}" data-issue-idx="${idx}">
      <td><span class="sev-badge" style="background:${color}">${escHtml(issue.severity)}</span></td>
      <td>${icon} ${escHtml((issue.type || '').replace('_', ' '))}</td>
      <td class="mono" style="font-size:11px">${escHtml(issue.rule_key || '')}</td>
      <td class="file-cell" title="${escAttr(issue.component_path || '')}"><span class="mono">${escHtml(file)}<span style="color:var(--text-muted)">:${loc}</span></span></td>
      <td>${escHtml(issue.message || '')}</td>
      <td class="actions-cell" onclick="event.stopPropagation()">${actionBtns}</td>
    </tr>`;
  }).join('');

  const hasMore = issues.length < total;
  const moreBtn = hasMore
    ? `<div style="text-align:center;padding:16px">
        <button class="btn btn-primary" id="loadMoreBtn" style="width:auto;padding:8px 24px">
          ${state.loadingIssues ? 'Loading\u2026' : `Load more (${total - issues.length} remaining)`}
        </button>
       </div>`
    : '';

  return summaryBar + filtersHtml + `
    <div class="issues-table-wrap">
      <table class="issues-table">
        <thead><tr>
          <th>Severity</th><th>Type</th><th>Rule</th><th>File</th><th>Message</th><th></th>
        </tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
    ${moreBtn}`;
}

function bindIssueControls() {
  document.querySelectorAll('.sev-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      const sev = btn.dataset.sev;
      state.issueFilter.severity = state.issueFilter.severity === sev ? 'all' : sev;
      loadIssues();
    });
  });
  document.getElementById('filterType')?.addEventListener('change', event => {
    state.issueFilter.type = event.target.value;
    loadIssues();
  });
  document.getElementById('filterStatus')?.addEventListener('change', event => {
    state.issueFilter.status = event.target.value;
    loadIssues();
  });
  let searchTimer;
  document.getElementById('filterSearch')?.addEventListener('input', event => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      state.issueFilter.search = event.target.value.trim();
      loadIssues();
    }, 300);
  });
  document.getElementById('loadMoreBtn')?.addEventListener('click', () => {
    state.issueOffset += ISSUE_PAGE;
    loadIssues(true);
  });

  document.querySelectorAll('.issues-table tbody tr[data-issue-idx]').forEach(row => {
    row.addEventListener('click', () => {
      const idx = Number.parseInt(row.dataset.issueIdx, 10);
      if (state.issues[idx]) openIssueDetail(state.issues[idx]);
    });
  });

  document.querySelectorAll('.itbtn').forEach(btn => {
    btn.addEventListener('click', async event => {
      event.stopPropagation();
      const id = btn.dataset.id;
      const res = btn.dataset.res;
      btn.disabled = true;
      try {
        await apiFetch('/issues/' + id + '/transition', {
          method: 'POST',
          body: JSON.stringify({ resolution: res, comment: '' }),
        });
        const idx = state.issues.findIndex(issue => String(issue.id) === String(id));
        if (idx !== -1) {
          const issue = state.issues[idx];
          if (res === '') {
            issue.status = 'open';
            issue.resolution = '';
          } else {
            issue.status = 'closed';
            issue.resolution = res;
          }
        }
        renderIssuesSection();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });
}

export function openIssueDetail(issue) {
  state.selectedIssue = issue;
  const inner = document.getElementById('detail-inner');
  const panel = document.getElementById('detail-panel');
  const overlay = document.getElementById('detail-overlay');
  if (!inner || !panel || !overlay) return;

  const currentIssue = issue;
  const file = (currentIssue.component_path || '').replace(/\\/g, '/');
  const loc = currentIssue.end_line && currentIssue.end_line !== currentIssue.line ? `${currentIssue.line}\u2013${currentIssue.end_line}` : `${currentIssue.line}`;
  const sevColor = SEV_COLOR[currentIssue.severity] || '#64748b';

  let secondaryHtml = '';
  if (currentIssue.secondary_locations && currentIssue.secondary_locations.length > 0) {
    const locs = currentIssue.secondary_locations.map((secondaryLocation, idx) => {
      const secondaryFile = (secondaryLocation.file_path || secondaryLocation.component_path || '').replace(/\\/g, '/').split('/').slice(-2).join('/');
      return `<div class="secondary-loc">
        <span class="loc-num">${idx + 1}</span>
        <span class="loc-file">${escHtml(secondaryFile)}:${secondaryLocation.line || ''}</span>
        <span class="loc-msg">${escHtml(secondaryLocation.message || '')}</span>
      </div>`;
    }).join('');
    secondaryHtml = `
      <div class="detail-section-title">Secondary Locations</div>
      ${locs}`;
  }

  inner.innerHTML = `
    <button class="detail-close" id="detailClose">\u2715</button>
    <div class="detail-title">${escHtml(currentIssue.message || 'Issue')}</div>
    <div class="detail-props">
      <div class="detail-prop">
        <span class="detail-prop-label">Severity</span>
        <span class="detail-prop-value"><span class="sev-badge" style="background:${sevColor}">${escHtml(currentIssue.severity)}</span></span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">Type</span>
        <span class="detail-prop-value">${TYPE_ICON[currentIssue.type] || ''} ${escHtml(TYPE_LABEL[currentIssue.type] || currentIssue.type || '')}</span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">Rule</span>
        <span class="detail-prop-value mono" style="font-size:12px">${escHtml(currentIssue.rule_key || '')}</span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">Engine</span>
        <span class="detail-prop-value">${escHtml(currentIssue.engine_id || '\u2014')}</span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">File</span>
        <span class="detail-prop-value mono" style="font-size:12px;word-break:break-all">${escHtml(file)}</span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">Location</span>
        <span class="detail-prop-value mono">Line ${loc}${currentIssue.column ? ', Col ' + currentIssue.column : ''}</span>
      </div>
      <div class="detail-prop">
        <span class="detail-prop-label">Status</span>
        <span class="detail-prop-value">${escHtml(currentIssue.status || 'open')}${currentIssue.resolution ? ' \u2014 ' + escHtml(currentIssue.resolution) : ''}</span>
      </div>
      ${currentIssue.tags && currentIssue.tags.length ? `<div class="detail-prop">
        <span class="detail-prop-label">Tags</span>
        <span class="detail-prop-value">${currentIssue.tags.map(tag => `<span class="tag">${escHtml(tag)}</span>`).join(' ')}</span>
      </div>` : ''}
    </div>
    ${secondaryHtml}
    <div id="rule-detail-section"><div class="loading-state" style="padding:16px 0"><div class="spinner"></div> Loading rule details\u2026</div></div>`;

  panel.classList.remove('hidden');
  overlay.classList.remove('hidden');
  requestAnimationFrame(() => {
    panel.classList.add('open');
    overlay.classList.add('open');
  });

  document.getElementById('detailClose').addEventListener('click', closeIssueDetail);
  overlay.addEventListener('click', closeIssueDetail);

  if (currentIssue.rule_key) {
    apiFetch(`/rules/${encodeURIComponent(currentIssue.rule_key)}`).then(rule => {
      const el = document.getElementById('rule-detail-section');
      if (!el) return;
      let html = '';
      if (rule.rationale) {
        html += `<div class="detail-section-title">Why is this a problem?</div>
          <div class="rule-rationale">${escHtml(rule.rationale)}</div>`;
      }
      if (rule.description && rule.description !== rule.rationale) {
        html += `<div class="detail-section-title">Description</div>
          <div class="rule-rationale">${escHtml(rule.description)}</div>`;
      }
      if (rule.noncompliant_code) {
        html += `<div class="detail-section-title">\u2718 Noncompliant Code</div>
          <pre class="rule-code noncompliant"><code>${escHtml(rule.noncompliant_code)}</code></pre>`;
      }
      if (rule.compliant_code) {
        html += `<div class="detail-section-title">\u2714 Compliant Code</div>
          <pre class="rule-code compliant"><code>${escHtml(rule.compliant_code)}</code></pre>`;
      }
      el.innerHTML = html || '';
    }).catch(() => {
      const el = document.getElementById('rule-detail-section');
      if (el) el.innerHTML = '';
    });
  }
}

export function closeIssueDetail() {
  const panel = document.getElementById('detail-panel');
  const overlay = document.getElementById('detail-overlay');
  if (!panel || !overlay) return;
  panel.classList.remove('open');
  overlay.classList.remove('open');
  setTimeout(() => {
    panel.classList.add('hidden');
    overlay.classList.add('hidden');
  }, 250);
  state.selectedIssue = null;
}