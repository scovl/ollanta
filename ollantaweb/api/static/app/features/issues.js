import { apiFetch } from '../core/api.js';
import { buildScopeQuery } from '../core/scope.js';
import {
  ISSUE_PAGE,
  SEV_BG,
  SEV_COLOR,
  SEV_LABEL,
  TYPE_ICON,
  TYPE_LABEL,
  QUALITY_LABEL,
  state,
} from '../core/state.js';
import { escAttr, escHtml } from '../core/utils.js';

let showToastMessage = () => {};

const TRACKING_LABEL = {
  new: 'New',
  reopened: 'Reopened',
  unchanged: 'Existing',
  unknown: 'Current',
};

const TRACKING_CLASS = {
  new: 'track-new',
  reopened: 'track-reopened',
  unchanged: 'track-unchanged',
  unknown: 'track-unknown',
};

const FACET_LIMIT = 8;

const FACET_GROUPS = [
  { title: 'Software Quality', field: 'quality', facet: 'by_quality', labels: QUALITY_LABEL },
  { title: 'Severity', field: 'severity', facet: 'by_severity', labels: SEV_LABEL },
  { title: 'Lifecycle', field: 'trackingState', facet: 'by_lifecycle', labels: TRACKING_LABEL },
  { title: 'Type', field: 'type', facet: 'by_type', labels: TYPE_LABEL },
  { title: 'Status', field: 'status', facet: 'by_status' },
  { title: 'Security Category', field: 'securityCategory', facet: 'by_security_category' },
  { title: 'Language', field: 'language', facet: 'by_language' },
  { title: 'Rule', field: 'rule', facet: 'by_rule' },
  { title: 'Tag', field: 'tag', facet: 'by_tags' },
  { title: 'Directory', field: 'directory', facet: 'by_directory' },
  { title: 'File', field: 'file', facet: 'by_file' },
];

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
  const qs = buildIssuesQueryParams(filter, {
    project_key: project.key,
    limit: String(ISSUE_PAGE),
    offset: String(state.issueOffset),
  });
  buildScopeQuery(state.scope).forEach((value, key) => qs.set(key, value));

  const facetQs = buildIssuesQueryParams(filter, { project_key: project.key });
  buildScopeQuery(state.scope).forEach((value, key) => facetQs.set(key, value));

  try {
    const [data, facets] = await Promise.all([
      apiFetch('/issues?' + qs.toString()),
      apiFetch('/issues/facets?' + facetQs.toString()),
    ]);
    if (append) {
      state.issues = state.issues.concat(data.items || []);
    } else {
      state.issues = data.items || [];
    }
    state.issuesTotal = data.total || 0;
    state.issueFacets = facets || null;
  } catch {
    if (!append) state.issues = [];
  }

  state.loadingIssues = false;
  renderIssuesSection();
}

function buildIssuesQueryParams(filter, initial) {
  const qs = new URLSearchParams(initial);
  if (filter.quality !== 'all') qs.set('quality', filter.quality);
  if (filter.severity !== 'all') qs.set('severity', filter.severity);
  if (filter.type !== 'all') qs.set('type', filter.type);
  if (filter.status !== 'all') qs.set('status', filter.status);
  if (filter.trackingState !== 'all') qs.set('tracking_state', filter.trackingState);
  if (filter.language !== 'all') qs.set('language', filter.language);
  if (filter.rule !== 'all') qs.set('rule_key', filter.rule);
  if (filter.tag !== 'all') qs.set('tag', filter.tag);
  if (filter.securityCategory !== 'all') qs.set('security_category', filter.securityCategory);
  if (filter.directory !== 'all') qs.set('directory', filter.directory);
  if (filter.file !== 'all') qs.set('file', filter.file);
  if (filter.file === 'all' && filter.search) qs.set('file', filter.search);
  if (filter.scanId) qs.set('scan_id', filter.scanId);
  return qs;
}

export function renderIssuesSection() {
  const el = document.getElementById('issues-section');
  if (!el) return;
  el.innerHTML = buildIssuesHtml();
  bindIssueControls();
}

function buildIssuesHtml() {
  const issues = state.issues;
  const total = state.issuesTotal;
  const filter = state.issueFilter;

  const facets = state.issueFacets || {};
  const facetSidebar = renderFacetSidebar(facets, filter);

  const filtersHtml = `
    <div class="issues-toolbar">
      <span class="section-title" style="margin:0">Issues
        <span style="font-size:13px;font-weight:400;color:var(--text-muted)">&nbsp;${total.toLocaleString()} total</span>
      </span>
      <div class="issues-filters">
        ${filter.scanId ? `<button type="button" class="filter-chip filter-chip-removable" id="clearScanFilterChip" title="Clear scan filter">Scan #${escHtml(filter.scanId)} \u00D7</button>` : ''}
        <input id="filterSearch" class="filter-input" type="text" placeholder="Search file or message\u2026" value="${escAttr(filter.search)}">
        <button class="btn-sm btn-outline" id="saveCurrentIssueFilterBtn">Save filter</button>
      </div>
    </div>`;

  if (state.loadingIssues && issues.length === 0) {
    return `<div class="issues-layout">${facetSidebar}<section class="issues-results">${filtersHtml}<div class="loading-state"><div class="spinner"></div></div></section></div>`;
  }

  if (issues.length === 0) {
    const isScanFilter = filter.scanId != null && filter.scanId !== '';
    const emptyMsg = isScanFilter
      ? `<p>No issues remain from analysis #${escHtml(filter.scanId)}.</p><p class="muted" style="margin-top:8px">They may have been resolved or removed in subsequent scans.</p><button type="button" class="btn-sm btn-outline" id="clearScanFilterEmptyBtn" style="margin-top:16px">Show all issues</button>`
      : `<p>No issues match the current filters.</p>`;
    return `<div class="issues-layout">${facetSidebar}<section class="issues-results">${filtersHtml}<div class="empty-state" style="padding:32px 0">${emptyMsg}</div></section></div>`;
  }

  const rows = issues.map((issue, idx) => {
    const color = SEV_COLOR[issue.severity] || '#64748b';
    const bg = SEV_BG[issue.severity] || 'transparent';
    const icon = TYPE_ICON[issue.type] || '?';
    const file = (issue.component_path || '').replaceAll('\\', '/').split('/').slice(-3).join('/');
    const loc = issue.end_line && issue.end_line !== issue.line ? `${issue.line}\u2013${issue.end_line}` : `${issue.line}`;
    const status = issue.status || 'open';
    const trackingState = issue.tracking_state || 'unknown';
    const trackingLabel = TRACKING_LABEL[trackingState] || 'Current';
    const trackingClass = TRACKING_CLASS[trackingState] || 'track-unknown';
    const isClosed = status === 'closed';
    let actionBtns = '';
    if (isClosed) {
      actionBtns = `<button class="itbtn re-btn" data-id="${issue.id}" data-res="" title="Reopen">\u21A9</button>`;
    } else {
      actionBtns = `
        <button class="itbtn fp-btn" data-id="${issue.id}" data-res="false_positive" title="False positive">FP</button>
        <button class="itbtn wf-btn" data-id="${issue.id}" data-res="wont_fix" title="Won\u2019t fix">WF</button>
        <button class="itbtn ok-btn" data-id="${issue.id}" data-res="fixed" title="Mark as fixed">\u2713</button>`;
    }
    return `<tr style="--row-sev-color:${color};--row-sev-bg:${bg}" class="sev-row${isClosed ? ' row-closed' : ''}" data-issue-idx="${idx}">
      <td><span class="sev-badge" style="background:${color}">${escHtml(issue.severity)}</span></td>
      <td>${icon} ${escHtml((issue.type || '').replace('_', ' '))}</td>
      <td><span class="issue-track ${trackingClass}">${escHtml(trackingLabel)}</span></td>
      <td class="mono" style="font-size:11px">${escHtml(issue.rule_key || '')}</td>
      <td class="file-cell" title="${escAttr(issue.component_path || '')}"><span class="mono">${escHtml(file)}<span style="color:var(--text-muted)">:${loc}</span></span></td>
      <td>${escHtml(issue.message || '')}</td>
      <td class="actions-cell" onclick="event.stopPropagation()">${actionBtns}</td>
    </tr>`;
  }).join('');

  const hasMore = issues.length < total;
  const loadMoreLabel = state.loadingIssues ? 'Loading\u2026' : `Load more (${total - issues.length} remaining)`;
  const moreBtn = hasMore
    ? `<div style="text-align:center;padding:16px">
        <button class="btn btn-primary" id="loadMoreBtn" style="width:auto;padding:8px 24px">
          ${loadMoreLabel}
        </button>
       </div>`
    : '';

  return `<div class="issues-layout">
    ${facetSidebar}
    <section class="issues-results">
    ${filtersHtml}
    <div class="issues-table-wrap">
      <table class="issues-table">
        <thead><tr>
          <th>Severity</th><th>Type</th><th>Lifecycle</th><th>Rule</th><th>File</th><th>Message</th><th></th>
        </tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
    ${moreBtn}
    </section>
  </div>`;
}

function renderFacetSidebar(facets, filter) {
  const groups = FACET_GROUPS.map(group => renderFacetGroup(group, facets[group.facet] || {}, filter)).join('');
  return `<aside class="issue-facet-sidebar">${groups}</aside>`;
}

function renderFacetGroup(group, values, filter) {
  const entries = Object.entries(values)
    .filter(([, count]) => count > 0)
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  const search = (state.issueFacetSearch[group.field] || '').toLowerCase();
  const filtered = search ? entries.filter(([key]) => facetLabel(group, key).toLowerCase().includes(search) || key.toLowerCase().includes(search)) : entries;
  const expanded = Boolean(state.issueFacetExpanded[group.field]);
  const visible = expanded ? filtered : filtered.slice(0, FACET_LIMIT);
  const active = filter[group.field];
  const rows = visible.map(([key, count]) => {
    const selected = active === key;
    return `<button class="facet-option${selected ? ' active' : ''}" data-facet-field="${escAttr(group.field)}" data-facet-value="${escAttr(key)}">
      <span class="facet-label">${escHtml(facetLabel(group, key))}</span>
      <span class="facet-count">${count.toLocaleString()}</span>
    </button>`;
  }).join('');
  const searchInput = entries.length > FACET_LIMIT
    ? `<input class="facet-search" data-facet-search="${escAttr(group.field)}" placeholder="Search ${escAttr(group.title.toLowerCase())}\u2026" value="${escAttr(state.issueFacetSearch[group.field] || '')}">`
    : '';
  const toggleLabel = expanded ? 'Show less' : 'Show more (' + (filtered.length - FACET_LIMIT) + ')';
  const toggle = filtered.length > FACET_LIMIT
    ? `<button class="facet-more" data-facet-expand="${escAttr(group.field)}">${toggleLabel}</button>`
    : '';
  return `<section class="facet-group">
    <div class="facet-title">${escHtml(group.title)}</div>
    ${searchInput}
    <div class="facet-options">${rows || '<div class="facet-empty">No data</div>'}</div>
    ${toggle}
  </section>`;
}

function facetLabel(group, key) {
  return group.labels?.[key] || key;
}

function bindIssueControls() {
  document.querySelectorAll('[data-facet-field]').forEach(btn => {
    btn.addEventListener('click', () => {
      const field = btn.dataset.facetField;
      const value = btn.dataset.facetValue;
      state.issueFilter[field] = state.issueFilter[field] === value ? 'all' : value;
      loadIssues();
    });
  });
  document.querySelectorAll('[data-facet-expand]').forEach(btn => {
    btn.addEventListener('click', () => {
      const field = btn.dataset.facetExpand;
      state.issueFacetExpanded[field] = !state.issueFacetExpanded[field];
      renderIssuesSection();
    });
  });
  document.querySelectorAll('[data-facet-search]').forEach(input => {
    input.addEventListener('input', event => {
      state.issueFacetSearch[input.dataset.facetSearch] = event.target.value;
      renderIssuesSection();
    });
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
  document.getElementById('saveCurrentIssueFilterBtn')?.addEventListener('click', saveCurrentIssueFilter);

  document.getElementById('clearScanFilterChip')?.addEventListener('click', () => {
    state.issueFilter.scanId = '';
    loadIssues();
  });
  document.getElementById('clearScanFilterEmptyBtn')?.addEventListener('click', () => {
    state.issueFilter.scanId = '';
    loadIssues();
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
  const file = (currentIssue.component_path || '').replaceAll('\\', '/');
  const loc = currentIssue.end_line && currentIssue.end_line !== currentIssue.line ? `${currentIssue.line}\u2013${currentIssue.end_line}` : `${currentIssue.line}`;
  const sevColor = SEV_COLOR[currentIssue.severity] || '#64748b';

  let secondaryHtml = '';
  if (currentIssue.secondary_locations && currentIssue.secondary_locations.length > 0) {
    const locs = currentIssue.secondary_locations.map((secondaryLocation, idx) => {
      const secondaryFile = (secondaryLocation.file_path || secondaryLocation.component_path || '').replaceAll('\\', '/').split('/').slice(-2).join('/');
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
    <button class="detail-copy" id="detailCopy" title="Copy issue details to clipboard">\uD83D\uDCCB Copy</button>
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
      <div class="detail-prop">
        <span class="detail-prop-label">Lifecycle</span>
        <span class="detail-prop-value">${escHtml(TRACKING_LABEL[currentIssue.tracking_state || 'unknown'] || 'Current')}</span>
      </div>
      ${currentIssue.tags?.length ? `<div class="detail-prop">
        <span class="detail-prop-label">Tags</span>
        <span class="detail-prop-value">${currentIssue.tags.map(tag => `<button class="tag tag-button" data-issue-tag="${escAttr(tag)}">${escHtml(tag)}</button>`).join(' ')}</span>
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
  document.getElementById('detailCopy')?.addEventListener('click', () => copyIssueDetail(currentIssue));
  overlay.addEventListener('click', closeIssueDetail);
  document.querySelectorAll('[data-issue-tag]').forEach(btn => {
    btn.addEventListener('click', event => {
      event.stopPropagation();
      state.issueFilter.tag = btn.dataset.issueTag;
      closeIssueDetail();
      loadIssues();
    });
  });

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

async function copyIssueDetail(issue) {
  const file = (issue.component_path || '').replaceAll('\\', '/');
  const loc = issue.end_line && issue.end_line !== issue.line
    ? `lines ${issue.line}\u2013${issue.end_line}`
    : `line ${issue.line}`;
  const sev = SEV_LABEL[issue.severity] || issue.severity || '';
  const type = (TYPE_LABEL[issue.type] || issue.type || 'issue');
  const rule = issue.rule_key || '';
  const engine = issue.engine_id || '';
  const status = issue.status || 'open';
  const resolution = issue.resolution ? ` (${issue.resolution})` : '';
  const lifecycle = TRACKING_LABEL[issue.tracking_state] || 'current';
  const tags = (issue.tags || []).join(', ');
  const quality = QUALITY_LABEL[issue.quality] || '';

  const lines = [];
  lines.push(`Issue: ${issue.message || 'Untitled'}`);
  if (sev) lines.push(`Severity: ${sev}`);
  if (type) lines.push(`Type: ${type}`);
  if (quality) lines.push(`Quality domain: ${quality}`);
  if (rule) lines.push(`Rule: ${rule}`);
  if (engine) lines.push(`Engine: ${engine}`);
  lines.push(`File: ${file}`);
  lines.push(`Location: ${loc}${issue.column ? ', column ' + issue.column : ''}`);
  lines.push(`Status: ${status}${resolution}`);
  lines.push(`Lifecycle: ${lifecycle}`);
  if (tags) lines.push(`Tags: ${tags}`);

  if (rule) {
    try {
      const ruleData = await apiFetch(`/rules/${encodeURIComponent(rule)}`);
      if (ruleData.rationale) lines.push(`\nWhy is this a problem?\n${ruleData.rationale}`);
      if (ruleData.noncompliant_code) lines.push(`\nNoncompliant code:\n${ruleData.noncompliant_code}`);
      if (ruleData.compliant_code) lines.push(`\nCompliant code:\n${ruleData.compliant_code}`);
    } catch {
      // rule details unavailable; omit from copy
    }
  }

  try {
    await navigator.clipboard.writeText(lines.join('\n'));
    const btn = document.getElementById('detailCopy');
    if (btn) {
      btn.textContent = '\u2714 Copied';
      setTimeout(() => { btn.textContent = '\uD83D\uDCCB Copy'; }, 2000);
    }
  } catch {
    // clipboard unavailable; show feedback anyway
    const btn = document.getElementById('detailCopy');
    if (btn) {
      btn.textContent = '\u26A0 Failed';
      setTimeout(() => { btn.textContent = '\uD83D\uDCCB Copy'; }, 2000);
    }
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

async function saveCurrentIssueFilter() {
  const criteria = currentIssueFilterCriteria();
  const project = state.currentProject?.key || 'project';
  const tag = criteria.tag ? ' ' + criteria.tag : '';
  try {
    await apiFetch('/saved-filters', {
      method: 'POST',
      body: JSON.stringify({
        name: project + tag + ' issues',
        visibility: 'private',
        filter_type: 'issues',
        criteria,
      }),
    });
    showToastMessage('Filter saved');
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

function currentIssueFilterCriteria() {
  const criteria = {};
  const filter = state.issueFilter || {};
  for (const [key, value] of Object.entries(filter)) {
    if (value && value !== 'all') criteria[key] = value;
  }
  if (state.currentProject?.key) criteria.project_key = state.currentProject.key;
  return criteria;
}
