'use strict';

import { apiFetch } from '../core/api.js';
import { buildScopeQuery, resolvedBranchName } from '../core/scope.js';
import { state } from '../core/state.js';
import { escAttr, escHtml } from '../core/utils.js';
import { loadCodeFileData } from './code.js';

let renderView = () => {};
let switchTabFn = () => {};

export function configureSurvivedMutantsFeature(options) {
  renderView = options.render || renderView;
  switchTabFn = options.switchTab || switchTabFn;
}

function clearSurvivedMutantsData() {
  state.survivedMutantsData = null;
  state.survivedMutantsError = '';
  state.survivedMutantsFilters = { module: '', changedOnly: false };
  state.survivedMutantsSort = { field: 'file', dir: 'asc' };
}

export async function loadSurvivedMutants(projectKey) {
  clearSurvivedMutantsData();
  state.loadingSurvivedMutants = true;
  renderView();

  try {
    const scope = state.scope;
    const scopeQuery = buildScopeQuery(scope);
    const qs = scopeQuery.toString();
    const latestUrl = '/projects/' + encodeURIComponent(projectKey) + '/scans/latest' + (qs ? '?' + qs : '');
    const latest = await apiFetch(latestUrl);
    if (!latest || !latest.id) {
      state.survivedMutantsData = { mutants: [], total: 0 };
      state.loadingSurvivedMutants = false;
      renderView();
      return;
    }
    const data = await apiFetch('/scans/' + latest.id + '/survived-mutants');
    state.survivedMutantsData = data || { mutants: [], total: 0 };
  } catch (err) {
    state.survivedMutantsData = null;
    state.survivedMutantsError = err.message || 'Failed to load survived mutants.';
  }

  state.loadingSurvivedMutants = false;
  renderView();
}

function filteredMutants() {
  const data = state.survivedMutantsData;
  if (!data?.mutants) return [];
  let list = [...data.mutants];
  const filters = state.survivedMutantsFilters || {};
  if (filters.module) {
    list = list.filter(mutant => mutant.module === filters.module);
  }
  if (filters.changedOnly) {
    list = list.filter(mutant => mutant.changed_code);
  }
  const sort = state.survivedMutantsSort || { field: 'file', dir: 'asc' };
  list.sort((a, b) => {
    let cmp = 0;
    if (sort.field === 'file') {
      cmp = (a.file || '').localeCompare(b.file || '');
    } else if (sort.field === 'line') {
      cmp = (a.line || 0) - (b.line || 0);
    } else if (sort.field === 'module') {
      cmp = (a.module || '').localeCompare(b.module || '');
    }
    return sort.dir === 'desc' ? -cmp : cmp;
  });
  return list;
}

function uniqueModules() {
  const data = state.survivedMutantsData;
  if (!data?.mutants) return [];
  const seen = new Set();
  const modules = [];
  for (const mutant of data.mutants) {
    if (mutant.module && !seen.has(mutant.module)) {
      seen.add(mutant.module);
      modules.push(mutant.module);
    }
  }
  return modules.sort();
}

export function renderSurvivedMutantsTab() {
  const project = state.currentProject;
  if (!project) return '';

  if (state.loadingSurvivedMutants) {
    return `<div class="svm-page">
      <div class="svm-header">
        <div>
          <h2 class="svm-title">Survived Mutants</h2>
          <p class="svm-subtitle">Loading...</p>
        </div>
      </div>
      <div class="loading-state"><div class="spinner"></div></div>
    </div>`;
  }

  if (state.survivedMutantsError) {
    return `<div class="svm-page">
      <div class="svm-header">
        <div>
          <h2 class="svm-title">Survived Mutants</h2>
          <p class="svm-subtitle">An error occurred</p>
        </div>
      </div>
      <div class="empty-state compact">
        <p>${escHtml(state.survivedMutantsError)}</p>
        <p>Ensure the scanner runs with <code>-with-mutations</code> and mutation reports are collected.</p>
      </div>
    </div>`;
  }

  const data = state.survivedMutantsData;
  if (!data) {
    return `<div class="svm-page">
      <div class="svm-header">
        <div>
          <h2 class="svm-title">Survived Mutants</h2>
          <p class="svm-subtitle">Loading...</p>
        </div>
      </div>
      <div class="loading-state"><div class="spinner"></div></div>
    </div>`;
  }

  const list = filteredMutants();
  const modules = uniqueModules();
  const scopeLabel = state.scope?.type === 'pull_request'
    ? ('PR #' + escHtml(state.scope.pullRequestKey || ''))
    : escHtml(resolvedBranchName(state.scope));
  const hasAny = data.mutants && data.mutants.length > 0;

  if (!hasAny) {
    return `<div class="svm-page">
      <div class="svm-header">
        <div>
          <h2 class="svm-title">Survived Mutants</h2>
          <p class="svm-subtitle">No survived mutants detected on ${scopeLabel}</p>
        </div>
      </div>
      <div class="empty-state compact">
        <p>No survived mutants in this scan. All generated mutants were killed by tests.</p>
      </div>
    </div>`;
  }

  return `<div class="svm-page">
    <div class="svm-header">
      <div>
        <h2 class="svm-title">Survived Mutants</h2>
        <p class="svm-subtitle">${list.length} survived mutant${list.length !== 1 ? 's' : ''} on ${scopeLabel}</p>
      </div>
      ${renderSurvivedMutantsToolbar(modules)}
    </div>
    ${renderSurvivedMutantsTable(list)}
  </div>`;
}

function renderSurvivedMutantsToolbar(modules) {
  const filters = state.survivedMutantsFilters || { module: '', changedOnly: false };
  const moduleOptions = modules.map(m => `<option value="${escAttr(m)}"${m === filters.module ? ' selected' : ''}>${escHtml(m)}</option>`).join('');
  return `<div class="svm-toolbar">
    <div class="svm-filters">
      <label class="svm-filter-field">
        <span>Module</span>
        <select id="svmModuleFilter" class="filter-sel">
          <option value="">All modules</option>
          ${moduleOptions}
        </select>
      </label>
      <label class="svm-filter-field svm-toggle-field">
        <input type="checkbox" id="svmChangedCodeFilter"${filters.changedOnly ? ' checked' : ''}>
        <span>Changed code only</span>
      </label>
    </div>
  </div>`;
}

function renderSurvivedMutantsTable(list) {
  const sort = state.survivedMutantsSort || { field: 'file', dir: 'asc' };

  function sortIndicator(field) {
    if (sort.field !== field) return '';
    return sort.dir === 'asc' ? ' \u2191' : ' \u2193';
  }

  return `<div class="svm-table-wrap">
    <table class="svm-table">
      <thead>
        <tr>
          <th class="svm-col-file"><button class="svm-sort-btn" data-svm-sort="file">File${sortIndicator('file')}</button></th>
          <th class="svm-col-line"><button class="svm-sort-btn" data-svm-sort="line">Line${sortIndicator('line')}</button></th>
          <th class="svm-col-mutator">Mutator</th>
          <th class="svm-col-description">Description</th>
          <th class="svm-col-module"><button class="svm-sort-btn" data-svm-sort="module">Module${sortIndicator('module')}</button></th>
          <th class="svm-col-changed">Changed</th>
        </tr>
      </thead>
      <tbody>
        ${list.map(mutant => {
          const shortFile = (mutant.file || '').replaceAll('\\', '/').split('/').slice(-2).join('/');
          const fullFile = mutant.file || '';
          return `<tr class="svm-row">
            <td class="svm-cell-file" title="${escAttr(fullFile)}">
              <a href="#" class="svm-file-link mono" data-svm-file="${escAttr(fullFile)}" data-svm-line="${mutant.line || 1}">${escHtml(shortFile)}</a>
            </td>
            <td class="svm-cell-line mono">${mutant.line || 1}</td>
            <td class="svm-cell-mutator"><span class="svm-mutator-badge">${escHtml(mutant.mutator || '\u2014')}</span></td>
            <td class="svm-cell-description" title="${escAttr(mutant.description || '')}">${escHtml(mutant.description || '\u2014')}</td>
            <td class="svm-cell-module mono">${escHtml(mutant.module || '\u2014')}</td>
            <td class="svm-cell-changed">${mutant.changed_code ? '<span class="svm-changed-badge">changed</span>' : '<span class="svm-changed-badge svm-changed-no">\u2014</span>'}</td>
          </tr>`;
        }).join('')}
      </tbody>
    </table>
  </div>`;
}

export function bindSurvivedMutantsContent() {
  const moduleFilter = document.getElementById('svmModuleFilter');
  if (moduleFilter) {
    moduleFilter.addEventListener('change', () => {
      state.survivedMutantsFilters.module = moduleFilter.value;
      renderView();
    });
  }

  const changedFilter = document.getElementById('svmChangedCodeFilter');
  if (changedFilter) {
    changedFilter.addEventListener('change', () => {
      state.survivedMutantsFilters.changedOnly = changedFilter.checked;
      renderView();
    });
  }

  document.querySelectorAll('[data-svm-sort]').forEach(btn => {
    btn.addEventListener('click', () => {
      const field = btn.dataset.svmSort;
      const current = state.survivedMutantsSort || { field: 'file', dir: 'asc' };
      if (current.field === field) {
        state.survivedMutantsSort.dir = current.dir === 'asc' ? 'desc' : 'asc';
      } else {
        state.survivedMutantsSort = { field, dir: 'asc' };
      }
      renderView();
    });
  });

  document.querySelectorAll('[data-svm-file]').forEach(link => {
    link.addEventListener('click', event => {
      event.preventDefault();
      const file = link.dataset.svmFile;
      if (file) {
        state.codeSelectedPath = file;
        switchTabFn('coverage');
        loadCodeFileData(file);
      }
    });
  });
}
