import { apiFetch } from '../core/api.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtDate, fmtNum } from '../core/utils.js';
import { loadIssues } from './issues.js';

let renderView = () => {};
let showToastMessage = () => {};

export function configureTagsFeature(options) {
  renderView = options.render;
  showToastMessage = options.showToast;
}

export async function loadTagsData(options = {}) {
  state.view = 'tags';
  state.loadingTags = true;
  if (!options.silent) renderView();
  try {
    const [catalog, filters] = await Promise.all([
      apiFetch('/tags?limit=200'),
      apiFetch('/saved-filters?filter_type=issues'),
    ]);
    state.tagCatalogData = catalog;
    state.savedFiltersData = filters;
  } catch (err) {
    state.tagCatalogData = { items: [], total: 0, error: err.message };
    state.savedFiltersData = { items: [], total: 0 };
  }
  state.loadingTags = false;
  renderView();
}

export async function loadTagDetail(key) {
  state.selectedTagKey = key;
  state.tagDetailData = null;
  renderView();
  try {
    state.tagDetailData = await apiFetch('/tags/' + encodeURIComponent(key));
  } catch (err) {
    state.tagDetailData = { error: err.message };
  }
  renderView();
}

function tagSourceMeta(tag) {
  const status = tag.status || 'active';
  if (status === 'deprecated') return { key: 'deprecated', label: 'Deprecated' };
  const isAuto = status === 'discovered' && !tag.owner_name && !tag.description;
  if (isAuto) return { key: 'auto', label: 'Auto' };
  return { key: 'governed', label: 'Governed' };
}

function applyCatalogFilters(tags) {
  const search = (state.tagSearch || '').toLowerCase();
  const source = state.tagSourceFilter || 'all';
  return tags.filter(tag => {
    if (search) {
      const haystack = [tag.key, tag.display_name, tag.description, tag.owner_name].filter(Boolean).join(' ').toLowerCase();
      if (!haystack.includes(search)) return false;
    }
    if (source === 'all') return true;
    return tagSourceMeta(tag).key === source;
  });
}

function countBySource(tags) {
  const totals = { governed: 0, auto: 0, deprecated: 0 };
  for (const tag of tags) {
    totals[tagSourceMeta(tag).key]++;
  }
  return totals;
}

export function renderTagsPage() {
  const catalog = state.tagCatalogData || { items: [], total: 0 };
  const filters = state.savedFiltersData || { items: [] };
  const tags = catalog.items || [];
  const tab = state.tagsTab || 'catalog';
  const totals = countBySource(tags);

  return `<div class="tags-page">
    <div class="page-header tags-page-header">
      <div>
        <h2>Tags <span class="tags-count">${fmtNum(catalog.total || tags.length)}</span></h2>
        <p>Governed vocabulary for issues, projects and rules \u2014 drives ownership, saved review views, and repeatable investigations.</p>
      </div>
      <div class="tags-header-actions">
        <button class="btn-sm btn-outline" id="tagsBackBtn">Back to projects</button>
        <button class="btn-sm btn-primary" id="openCreateTagBtn">+ Create Tag</button>
      </div>
    </div>

    ${state.loadingTags ? '<div class="loading-state"><div class="spinner"></div></div>' : `
      ${renderHintBanner(totals)}
      <div class="tags-tabs" role="tablist">
        ${renderTagTabBtn('catalog', 'Catalog', tags.length)}
        ${renderTagTabBtn('bulk', 'Bulk Edit', state.tagBulkPreview?.target_count || null)}
        ${renderTagTabBtn('filters', 'Saved Filters', (filters.items || []).length)}
      </div>

      <section class="tag-tab-panel${tab === 'catalog' ? '' : ' hidden'}" data-tag-tab="catalog">
        ${renderCreateTagPanel()}
        ${renderCatalogPanel(tags, totals)}
        ${renderTagDetailPanel()}
      </section>

      <section class="tag-tab-panel${tab === 'bulk' ? '' : ' hidden'}" data-tag-tab="bulk">
        ${renderBulkEditPanel()}
      </section>

      <section class="tag-tab-panel${tab === 'filters' ? '' : ' hidden'}" data-tag-tab="filters">
        ${renderSavedFiltersPanel(filters.items || [])}
      </section>
    `}
  </div>`;
}

function renderTagTabBtn(id, label, badge) {
  const active = (state.tagsTab || 'catalog') === id;
  const badgeHtml = badge != null && badge !== '' ? `<span class="tag-tab-badge">${fmtNum(badge)}</span>` : '';
  return `<button class="tag-tab-btn${active ? ' active' : ''}" data-tag-tab-btn="${id}" role="tab" aria-selected="${active ? 'true' : 'false'}">${escHtml(label)}${badgeHtml}</button>`;
}

function renderHintBanner(totals) {
  if (!totals.auto) return '';
  return `<div class="tag-hint-banner">
    <span class="tag-hint-icon">\u24D8</span>
    <div>
      <strong>${fmtNum(totals.auto)} auto-discovered tag${totals.auto === 1 ? '' : 's'}</strong>
      were extracted from rules. Open one to add a description and owner \u2014 that promotes it to <em>Governed</em>.
    </div>
  </div>`;
}

function renderCreateTagPanel() {
  if (!state.tagCreatorOpen) return '';
  return `<section class="tag-panel tag-create-panel">
    <div class="tag-panel-header">
      <h3>Create Tag</h3>
      <button class="btn-sm btn-ghost" id="closeCreateTagBtn">Close</button>
    </div>
    <div class="tag-form-grid labeled-grid">
      <label class="form-field"><span>Key <em>required</em></span><input id="tagKeyInput" class="filter-input" placeholder="team-api"></label>
      <label class="form-field"><span>Display name</span><input id="tagNameInput" class="filter-input" placeholder="Team API"></label>
      <label class="form-field"><span>Color</span><input id="tagColorInput" class="filter-input" placeholder="#0ea5e9"></label>
      <label class="form-field"><span>Owner</span><input id="tagOwnerInput" class="filter-input" placeholder="Platform team"></label>
      <label class="form-field full"><span>Description</span><input id="tagDescriptionInput" class="filter-input" placeholder="What does this tag mean?"></label>
    </div>
    <div class="tag-create-actions">
      <button class="btn-sm btn-primary" id="createTagBtn">Create tag</button>
    </div>
  </section>`;
}

function renderCatalogPanel(tags, totals) {
  const filtered = applyCatalogFilters(tags);
  const source = state.tagSourceFilter || 'all';
  const sourceChip = (id, label, count) => `<button class="tag-source-chip${source === id ? ' active' : ''}" data-tag-source="${id}">${escHtml(label)} <span>${fmtNum(count)}</span></button>`;

  const rows = filtered.length ? filtered.map(tag => {
    const usage = tag.usage || {};
    const total = (usage.issue_count || 0) + (usage.project_count || 0) + (usage.rule_count || 0) + (usage.custom_rule_count || 0);
    const meta = tagSourceMeta(tag);
    const isSelected = state.selectedTagKey === tag.key;
    return `<tr class="${isSelected ? 'is-selected' : ''}" data-tag-key="${escAttr(tag.key)}">
      <td><button class="tag-link-btn" data-tag-open="${escAttr(tag.key)}">${renderTagChip(tag)}</button></td>
      <td class="cell-truncate" title="${escAttr(tag.description || '')}">${escHtml(tag.description || '\u2014')}</td>
      <td>${escHtml(tag.owner_name || '\u2014')}</td>
      <td><span class="tag-status tag-status-${escAttr(meta.key)} tag-status-${escAttr(tag.status || 'active')}">${escHtml(meta.label)}</span></td>
      <td class="num mono">${fmtNum(total)}</td>
      <td class="row-actions">
        <button class="btn-sm btn-ghost" data-tag-open="${escAttr(tag.key)}">Inspect</button>
        <button class="btn-sm btn-ghost" data-tag-filter="${escAttr(tag.key)}" title="Open issues filtered by this tag">Issues</button>
      </td>
    </tr>`;
  }).join('') : `<tr><td colspan="6"><div class="empty-state compact" style="padding:24px 0"><p>No tags match your filter.</p></div></td></tr>`;

  return `<section class="tag-panel">
    <div class="tag-panel-header">
      <h3>Catalog</h3>
      <div class="tag-panel-tools">
        <input id="tagSearchInput" class="filter-input" placeholder="Search by key, name, owner\u2026" value="${escAttr(state.tagSearch || '')}">
      </div>
    </div>
    <div class="tag-source-chips">
      ${sourceChip('all', 'All', tags.length)}
      ${sourceChip('governed', 'Governed', totals.governed)}
      ${sourceChip('auto', 'Auto', totals.auto)}
      ${sourceChip('deprecated', 'Deprecated', totals.deprecated)}
    </div>
    <div class="tags-table-wrap">
      <table class="tags-table">
        <thead><tr><th>Tag</th><th>Description</th><th>Owner</th><th>Source</th><th class="num">Usage</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  </section>`;
}

function renderTagDetailPanel() {
  if (!state.selectedTagKey) return '';
  const detail = state.tagDetailData;
  if (!detail) return `<section class="tag-panel tag-detail-panel"><div class="tag-panel-header"><h3>${escHtml(state.selectedTagKey)}</h3><button class="btn-sm btn-ghost" id="closeTagDetailBtn">Close</button></div><div class="loading-state"><div class="spinner"></div></div></section>`;
  if (detail.error) return `<section class="tag-panel tag-detail-panel"><div class="tag-panel-header"><h3>${escHtml(state.selectedTagKey)}</h3><button class="btn-sm btn-ghost" id="closeTagDetailBtn">Close</button></div><div class="error-msg">${escHtml(detail.error)}</div></section>`;
  const tag = detail.tag || {};
  const usage = tag.usage || {};
  const meta = tagSourceMeta(tag);
  const aliases = (tag.aliases || []).map(alias => `<span class="tag-mini">${escHtml(alias.alias)}</span>`).join('') || '<span class="muted">None</span>';
  const auditRows = (detail.audit || []).slice(0, 8).map(entry => `<div class="tag-audit-row">
    <span>${escHtml(entry.action)}</span>
    <span class="muted">${fmtDate(entry.created_at)}</span>
  </div>`).join('') || '<div class="muted">No audit entries yet.</div>';

  const promoteHint = meta.key === 'auto'
    ? `<div class="tag-promote-hint">This tag is auto-discovered. Add a description and owner to make it Governed.</div>`
    : '';

  return `<section class="tag-panel tag-detail-panel">
    <div class="tag-panel-header">
      <div class="tag-detail-title">${renderTagChip(tag)} <span class="tag-status tag-status-${escAttr(meta.key)}">${escHtml(meta.label)}</span></div>
      <button class="btn-sm btn-ghost" id="closeTagDetailBtn">Close</button>
    </div>
    <p class="tag-detail-description">${escHtml(tag.description || 'No description')}</p>
    ${promoteHint}
    <div class="tag-meta-grid">
      <div><span>Owner</span><strong>${escHtml(tag.owner_name || 'Unassigned')}</strong></div>
      <div><span>Status</span><strong>${escHtml(tag.status || 'active')}</strong></div>
      <div><span>Issues</span><strong>${fmtNum(usage.issue_count || 0)}</strong></div>
      <div><span>Projects</span><strong>${fmtNum(usage.project_count || 0)}</strong></div>
      <div><span>Rules</span><strong>${fmtNum((usage.rule_count || 0) + (usage.custom_rule_count || 0))}</strong></div>
      <div><span>Filters</span><strong>${fmtNum(usage.saved_filter_count || 0)}</strong></div>
    </div>
    <div class="tag-detail-section"><span class="detail-section-title">Aliases</span><div class="tag-aliases">${aliases}</div></div>
    ${meta.key === 'auto' ? `<div class="tag-detail-section">
      <span class="detail-section-title">Promote to Governed</span>
      <div class="tag-form-grid labeled-grid promote-form">
        <label class="form-field"><span>Description</span><input id="promoteDescription" class="filter-input" placeholder="Describe what this tag means"></label>
        <label class="form-field"><span>Owner</span><input id="promoteOwner" class="filter-input" placeholder="Owning team"></label>
      </div>
      <button class="btn-sm btn-primary" data-tag-promote="${escAttr(tag.key)}">Save</button>
    </div>` : ''}
    <div class="tag-detail-actions">
      <button class="btn-sm btn-outline" data-tag-filter="${escAttr(tag.key)}">Open issues</button>
      <button class="btn-sm btn-ghost" data-tag-deprecate="${escAttr(tag.key)}">Deprecate</button>
    </div>
    <div class="tag-detail-section"><span class="detail-section-title">Recent Activity</span>${auditRows}</div>
  </section>`;
}

function renderBulkEditPanel() {
  const preview = state.tagBulkPreview;
  const changes = preview?.changes?.slice(0, 10).map(change => `<div class="tag-bulk-change">
    <span class="mono">${escHtml(change.target_key || String(change.target_id || 'target'))}</span>
    <span class="tag-bulk-arrow">${(change.before || []).map(tag => `<span class="tag-mini">${escHtml(tag)}</span>`).join(' ') || '<span class="muted">none</span>'} \u2192 ${(change.after || []).map(tag => `<span class="tag-mini">${escHtml(tag)}</span>`).join(' ') || '<span class="muted">none</span>'}</span>
  </div>`).join('') || '';
  const validation = preview?.validation_errors?.length ? `<div class="error-msg">${escHtml(preview.validation_errors.join(', '))}</div>` : '';
  const previewHtml = preview ? `<div class="tag-bulk-preview"><strong>${fmtNum(preview.target_count || 0)} target${(preview.target_count || 0) === 1 ? '' : 's'} affected</strong>${validation}<div class="tag-bulk-changes">${changes}</div></div>` : '';

  return `<section class="tag-panel">
    <div class="tag-panel-header">
      <h3>Bulk edit</h3>
      <p class="tag-panel-subtitle">Add or remove tags from many entities at once. Always preview before applying.</p>
    </div>
    <div class="tag-form-grid labeled-grid bulk-form">
      <label class="form-field"><span>Target type</span>
        <select id="bulkTargetType" class="filter-sel">
          <option value="issue">Issues</option>
          <option value="project">Projects</option>
          <option value="custom_rule">Custom rules</option>
          <option value="rule">Rules</option>
        </select>
      </label>
      <label class="form-field"><span>Target IDs</span><input id="bulkTargetIds" class="filter-input" placeholder="123, 456, 789"></label>
      <label class="form-field"><span>Target keys</span><input id="bulkTargetKeys" class="filter-input" placeholder="ISSUE-1, ISSUE-2"></label>
      <label class="form-field"><span>Add tags</span><input id="bulkAddTags" class="filter-input" placeholder="team-api, backend"></label>
      <label class="form-field"><span>Remove tags</span><input id="bulkRemoveTags" class="filter-input" placeholder="legacy"></label>
    </div>
    <div class="tag-bulk-actions">
      <button class="btn-sm btn-outline" id="previewBulkTagsBtn">Preview</button>
      <button class="btn-sm btn-primary" id="applyBulkTagsBtn">Apply</button>
    </div>
    ${previewHtml}
  </section>`;
}

function renderSavedFiltersPanel(filters) {
  const rows = filters.length ? filters.map(filter => `<div class="saved-filter-row">
    <div>
      <strong>${escHtml(filter.name)}</strong>
      <span class="muted">${escHtml(formatCriteria(filter.criteria || {}))}</span>
    </div>
    <button class="btn-sm btn-outline apply-saved-filter-btn" data-filter-id="${filter.id}">Apply</button>
  </div>`).join('') : '<div class="empty-state compact"><p>No saved filters yet. Create one below.</p></div>';

  return `<section class="tag-panel saved-filter-panel">
    <div class="tag-panel-header">
      <h3>Saved Filters</h3>
      <p class="tag-panel-subtitle">Reusable issue queries that appear across the platform.</p>
    </div>
    <div class="saved-filter-list">${rows}</div>
    <div class="tag-detail-section">
      <span class="detail-section-title">Create filter</span>
      <div class="tag-form-grid labeled-grid">
        <label class="form-field"><span>Filter name</span><input id="savedFilterName" class="filter-input" placeholder="Critical security issues"></label>
        <label class="form-field"><span>Tag key</span><input id="savedFilterTag" class="filter-input" placeholder="team-api"></label>
        <label class="form-field"><span>Visibility</span>
          <select id="savedFilterVisibility" class="filter-sel">
            <option value="private">Private</option>
            <option value="shared">Shared</option>
          </select>
        </label>
      </div>
      <button class="btn-sm btn-primary" id="saveTagFilterBtn">Save filter</button>
    </div>
  </section>`;
}

function renderTagChip(tag) {
  const color = tag.color || '#64748b';
  return `<span class="tag-chip" style="--tag-color:${escAttr(color)}"><span class="tag-swatch"></span>${escHtml(tag.display_name || tag.key || '')}</span>`;
}

function formatCriteria(criteria) {
  const entries = Object.entries(criteria);
  if (!entries.length) return 'No criteria';
  return entries.map(([key, value]) => `${key}: ${Array.isArray(value) ? value.join(',') : value}`).join(' \u00B7 ');
}

export function bindTagsPage() {
  document.querySelectorAll('[data-tag-tab-btn]').forEach(btn => btn.addEventListener('click', () => {
    state.tagsTab = btn.dataset.tagTabBtn;
    renderView();
  }));
  document.getElementById('openCreateTagBtn')?.addEventListener('click', () => {
    state.tagCreatorOpen = !state.tagCreatorOpen;
    state.tagsTab = 'catalog';
    renderView();
  });
  document.getElementById('closeCreateTagBtn')?.addEventListener('click', () => {
    state.tagCreatorOpen = false;
    renderView();
  });
  document.getElementById('createTagBtn')?.addEventListener('click', createTagFromForm);
  document.querySelectorAll('[data-tag-open]').forEach(btn => btn.addEventListener('click', () => loadTagDetail(btn.dataset.tagOpen)));
  document.getElementById('closeTagDetailBtn')?.addEventListener('click', () => {
    state.selectedTagKey = '';
    state.tagDetailData = null;
    renderView();
  });
  document.querySelectorAll('[data-tag-source]').forEach(btn => btn.addEventListener('click', () => {
    state.tagSourceFilter = btn.dataset.tagSource;
    renderView();
  }));
  document.getElementById('tagSearchInput')?.addEventListener('input', event => {
    state.tagSearch = event.target.value.trim();
    renderView();
  });
  document.querySelectorAll('[data-tag-filter]').forEach(btn => btn.addEventListener('click', () => openTagIssues(btn.dataset.tagFilter)));
  document.querySelectorAll('[data-tag-deprecate]').forEach(btn => btn.addEventListener('click', () => deprecateTag(btn.dataset.tagDeprecate)));
  document.querySelectorAll('[data-tag-promote]').forEach(btn => btn.addEventListener('click', () => promoteTag(btn.dataset.tagPromote)));
  document.getElementById('previewBulkTagsBtn')?.addEventListener('click', () => submitBulkEdit(false));
  document.getElementById('applyBulkTagsBtn')?.addEventListener('click', () => submitBulkEdit(true));
  document.getElementById('saveTagFilterBtn')?.addEventListener('click', saveFilterFromForm);
  document.querySelectorAll('.apply-saved-filter-btn').forEach(btn => btn.addEventListener('click', () => applySavedFilter(btn.dataset.filterId)));
}

async function createTagFromForm() {
  const body = {
    key: valueOf('tagKeyInput'),
    display_name: valueOf('tagNameInput'),
    color: valueOf('tagColorInput'),
    owner_name: valueOf('tagOwnerInput'),
    description: valueOf('tagDescriptionInput'),
  };
  if (!body.key) {
    showToastMessage('Key is required', 'error');
    return;
  }
  try {
    const tag = await apiFetch('/tags', { method: 'POST', body: JSON.stringify(body) });
    showToastMessage('Tag created');
    state.selectedTagKey = tag.key;
    state.tagCreatorOpen = false;
    await loadTagsData({ silent: true });
    await loadTagDetail(tag.key);
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function deprecateTag(key) {
  try {
    await apiFetch('/tags/' + encodeURIComponent(key) + '/deprecate', { method: 'POST', body: JSON.stringify({}) });
    showToastMessage('Tag deprecated');
    await loadTagsData({ silent: true });
    await loadTagDetail(key);
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function promoteTag(key) {
  const description = valueOf('promoteDescription');
  const owner = valueOf('promoteOwner');
  if (!description && !owner) {
    showToastMessage('Add a description or owner to promote', 'error');
    return;
  }
  try {
    await apiFetch('/tags/' + encodeURIComponent(key), {
      method: 'PATCH',
      body: JSON.stringify({ description, owner_name: owner, status: 'active' }),
    });
    showToastMessage('Tag promoted');
    await loadTagsData({ silent: true });
    await loadTagDetail(key);
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function submitBulkEdit(apply) {
  const body = bulkEditBody();
  try {
    const result = await apiFetch('/tags/bulk/' + (apply ? 'apply' : 'preview'), { method: 'POST', body: JSON.stringify(body) });
    state.tagBulkPreview = result;
    showToastMessage(apply ? 'Bulk edit applied' : 'Preview ready');
    await loadTagsData({ silent: true });
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function saveFilterFromForm() {
  const tag = valueOf('savedFilterTag');
  const name = valueOf('savedFilterName') || (tag ? 'Issues tagged ' + tag : 'Issue filter');
  try {
    await apiFetch('/saved-filters', {
      method: 'POST',
      body: JSON.stringify({ name, visibility: valueOf('savedFilterVisibility') || 'private', filter_type: 'issues', criteria: tag ? { tag } : {} }),
    });
    showToastMessage('Filter saved');
    await loadTagsData({ silent: true });
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function applySavedFilter(id) {
  try {
    const data = await apiFetch('/saved-filters/' + encodeURIComponent(id) + '/apply', { method: 'POST', body: JSON.stringify({}) });
    const criteria = data.criteria || {};
    state.issueFilter = { ...state.issueFilter, ...criteria };
    if (criteria.tag) state.issueFilter.tag = criteria.tag;
    showToastMessage('Filter applied');
  } catch (err) {
    showToastMessage(err.message, 'error');
  }
}

async function openTagIssues(tag) {
  if (!tag) return;
  state.issueFilter.tag = tag;
  if (state.currentProject?.key) {
    state.view = 'project';
    state.projectTab = 'issues';
    renderView();
    await loadIssues();
    return;
  }
  showToastMessage('Tag filter ready for issue views');
}

function bulkEditBody() {
  return {
    target_type: valueOf('bulkTargetType') || 'issue',
    target_ids: splitCSV(valueOf('bulkTargetIds')).map(value => Number.parseInt(value, 10)).filter(Number.isFinite),
    target_keys: splitCSV(valueOf('bulkTargetKeys')),
    add_tags: splitCSV(valueOf('bulkAddTags')),
    remove_tags: splitCSV(valueOf('bulkRemoveTags')),
  };
}

function splitCSV(value) {
  return String(value || '').split(',').map(item => item.trim()).filter(Boolean);
}

function valueOf(id) {
  return document.getElementById(id)?.value?.trim() || '';
}
