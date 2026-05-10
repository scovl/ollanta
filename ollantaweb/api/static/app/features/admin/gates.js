import { apiFetch } from '../../core/api.js';
import { state } from '../../core/state.js';
import { escAttr, escHtml } from '../../core/utils.js';
import { getRenderView, getShowToast } from './context.js';
import { conditionMetricSuffix, gateConditionLabel, gateMetricOptions, operatorLabel } from './shared.js';

export async function loadGateData() {
  try {
    const data = await apiFetch('/quality-gates');
    state.gateData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.gateData = [];
  }
  getRenderView()();
}

export function renderGateTab() {
  const gates = state.gateData;
  if (gates === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const rows = (gates || []).map(gate => `
    <div class="gate-card">
      <div class="gate-header">
        <div>
          <span class="gate-name">${escHtml(gate.name)}</span>
          ${gate.is_default ? `<span class="badge badge-ok" style="font-size:11px;margin-left:8px">Default</span>` : ''}
          ${gate.is_builtin ? `<span class="badge badge-warn" style="font-size:11px;margin-left:8px">Built-in</span>` : ''}
        </div>
        <div class="gate-actions">
          <button class="btn-sm btn-outline assign-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Assign to project</button>
          ${gate.is_builtin ? '' : `<button class="btn-sm btn-ghost set-default-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Set default</button>`}
          <button class="btn-sm btn-ghost copy-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Copy</button>
          ${gate.is_builtin ? '' : `<button class="btn-sm btn-danger del-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Delete</button>`}
          <button class="btn-sm btn-ghost expand-gate-btn" data-gate-id="${gate.id}">Conditions \u25BE</button>
        </div>
      </div>
      <div class="gate-conditions hidden" id="gate-cond-${gate.id}">
        <div class="loading-inline">Loading\u2026</div>
      </div>
    </div>`).join('');

  const emptyMsg = (gates || []).length ? '' : `<div class="empty-state" style="padding:20px 0"><p>No quality gates configured.</p></div>`;

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Quality Gates</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Conditions that must pass for a project analysis to be considered successful.</p>
    <div class="create-form" style="margin-bottom:20px">
      <div class="form-row">
        <input id="newGateName" class="filter-input" placeholder="Gate name" style="width:250px">
        <button class="btn btn-primary" id="createGateBtn" style="width:auto;padding:6px 18px;margin-top:0">Create gate</button>
      </div>
    </div>
    <div class="gate-list">${rows || ''}</div>
    ${emptyMsg}
  </div>`;
}

async function reloadGateConditions(gateId, box) {
  box.innerHTML = '<div class="loading-inline">Loading\u2026</div>';
  box.classList.remove('hidden');
  const expandBtn = document.querySelector('.expand-gate-btn[data-gate-id="' + gateId + '"]');
  if (expandBtn) expandBtn.textContent = 'Conditions \u25B4';
  try {
    const gate = await apiFetch('/quality-gates/' + gateId);
    const conds = gate.conditions || [];
    if (conds.length) {
      box.innerHTML = `<table class="conditions-table">
        <thead><tr><th>Metric</th><th>Operator</th><th>Value</th><th>Warning</th><th>New Code</th><th></th></tr></thead>
        <tbody>${conds.map(condition => `<tr>
          <td>${gateConditionLabel(condition.metric)}</td>
          <td>${escHtml(operatorLabel(condition.operator))}</td>
          <td class="mono">${conditionMetricSuffix(condition.threshold, condition.metric)}</td>
          <td class="mono">${condition.warning_threshold != null ? conditionMetricSuffix(condition.warning_threshold, condition.metric) : '\u2014'}</td>
          <td>${condition.on_new_code ? '\u2713' : ''}</td>
          <td><button class="btn-sm btn-danger del-cond-btn" data-cond-id="${condition.id}" data-gate-id="${gateId}">Remove</button></td>
        </tr>`).join('')}</tbody>
      </table>`;
    } else {
      box.innerHTML = '<p style="color:var(--text-muted);padding:8px 0;font-size:13px">No conditions defined.</p>';
    }
    box.innerHTML += renderAddConditionForm(gateId);
  } catch {
    box.innerHTML = '<p style="color:var(--danger);font-size:13px">Failed to reload conditions.</p>';
  }
}

function renderAddConditionForm(gateId) {
  return `<div class="create-form" style="padding-top:12px;border-top:1px solid var(--border)">
    <div class="form-row" style="gap:6px;flex-wrap:wrap">
      <select class="filter-sel add-cond-metric" data-gate-id="${gateId}" style="width:160px">
        ${gateMetricOptions()}
      </select>
      <select class="filter-sel add-cond-op" data-gate-id="${gateId}">
        <option value="LT">is less than</option>
        <option value="GT">is greater than</option>
        <option value="GTE">is greater than or equal</option>
        <option value="LTE">is less than or equal</option>
        <option value="EQ">equals</option>
        <option value="NE">is not equal</option>
      </select>
      <input class="filter-input add-cond-threshold" placeholder="value" type="number" step="any" style="width:80px" data-gate-id="${gateId}">
      <input class="filter-input add-cond-warning" placeholder="warning (opt)" type="number" step="any" style="width:110px" data-gate-id="${gateId}">
      <label style="font-size:12px;white-space:nowrap"><input type="checkbox" class="add-cond-newcode" data-gate-id="${gateId}"> On new code</label>
      <button class="btn-sm btn-primary add-cond-btn" data-gate-id="${gateId}">Add condition</button>
    </div>
  </div>`;
}

export function bindGatesContent() {
  const project = state.currentProject;
  if (!project) return;
  const showToast = getShowToast();

  document.querySelectorAll('.expand-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + id);
      if (!box) return;
      const hidden = box.classList.toggle('hidden');
      btn.textContent = hidden ? 'Conditions \u25BE' : 'Conditions \u25B4';
      if (hidden) return;

      if (box.innerHTML.includes('Loading')) {
        try {
          const gate = await apiFetch('/quality-gates/' + id);
          const conds = gate.conditions || [];
          if (conds.length) {
            box.innerHTML = `<table class="conditions-table">
              <thead><tr><th>Metric</th><th>Operator</th><th>Value</th><th>Warning</th><th>New Code</th><th></th></tr></thead>
              <tbody>${conds.map(condition => `<tr>
                <td>${gateConditionLabel(condition.metric)}</td>
                <td>${escHtml(operatorLabel(condition.operator))}</td>
                <td class="mono">${conditionMetricSuffix(condition.threshold, condition.metric)}</td>
                <td class="mono">${condition.warning_threshold !== null ? conditionMetricSuffix(condition.warning_threshold, condition.metric) : '\u2014'}</td>
                <td>${condition.on_new_code ? '\u2713' : ''}</td>
                <td><button class="btn-sm btn-danger del-cond-btn" data-cond-id="${condition.id}" data-gate-id="${id}">Remove</button></td>
              </tr>`).join('')}</tbody>
            </table>`;
          } else {
            box.innerHTML = '<p style="color:var(--text-muted);padding:8px 0;font-size:13px">No conditions defined.</p>';
          }
          box.innerHTML += renderAddConditionForm(id);
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
        showToast('Gate "' + name + '" assigned.');
      } catch (err) {
        showToast(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.getElementById('createGateBtn')?.addEventListener('click', async () => {
    const name = document.getElementById('newGateName')?.value.trim();
    if (!name) { showToast('Gate name is required.', 'error'); return; }
    try {
      await apiFetch('/quality-gates', {
        method: 'POST',
        body: JSON.stringify({ name, small_changeset_lines: 20 }),
      });
      showToast('Gate "' + name + '" created.');
      state.gateData = null;
      await loadGateData();
    } catch (err) {
      showToast(err.message, 'error');
    }
  });

  document.querySelectorAll('.del-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const name = btn.dataset.gateName;
      if (!confirm('Delete quality gate "' + name + '" and all its conditions?')) return;
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId, { method: 'DELETE' });
        showToast('Gate "' + name + '" deleted.');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToast(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.querySelectorAll('.set-default-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId + '/set-default', { method: 'POST' });
        showToast('Default gate set.');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToast(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.querySelectorAll('.copy-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const newName = prompt('Copy "' + btn.dataset.gateName + '" as:');
      if (!newName) return;
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId + '/copy', {
          method: 'POST',
          body: JSON.stringify({ name: newName }),
        });
        showToast('Gate copied as "' + newName + '".');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToast(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.addEventListener('click', async event => {
    const addBtn = event.target.closest('.add-cond-btn');
    if (addBtn) {
      const gateId = addBtn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + gateId);
      const metric = box?.querySelector('.add-cond-metric')?.value;
      const op = box?.querySelector('.add-cond-op')?.value || 'GT';
      const thresholdStr = box?.querySelector('.add-cond-threshold')?.value.trim();
      const warningStr = box?.querySelector('.add-cond-warning')?.value.trim();
      const onNewCode = box?.querySelector('.add-cond-newcode')?.checked || false;
      if (!metric) { showToast('Metric is required.', 'error'); return; }
      if (!thresholdStr) { showToast('Value is required.', 'error'); return; }
      const threshold = Number.parseFloat(thresholdStr);
      if (Number.isNaN(threshold)) { showToast('Value must be a number.', 'error'); return; }
      let warningThreshold = undefined;
      if (warningStr) {
        const w = Number.parseFloat(warningStr);
        if (!Number.isNaN(w)) warningThreshold = w;
      }
      addBtn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + gateId + '/conditions', {
          method: 'POST',
          body: JSON.stringify({ metric, operator: op, threshold, warning_threshold: warningThreshold, on_new_code: onNewCode }),
        });
        showToast('Condition added.');
        await reloadGateConditions(gateId, box);
      } catch (err) {
        showToast(err.message, 'error');
        addBtn.disabled = false;
      }
      return;
    }
    const delBtn = event.target.closest('.del-cond-btn');
    if (delBtn) {
      const gateId = delBtn.dataset.gateId;
      const condId = delBtn.dataset.condId;
      if (!confirm('Remove this condition?')) return;
      delBtn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + gateId + '/conditions/' + condId, { method: 'DELETE' });
        showToast('Condition removed.');
        const box = document.getElementById('gate-cond-' + gateId);
        if (box) await reloadGateConditions(gateId, box);
      } catch (err) {
        showToast(err.message, 'error');
        delBtn.disabled = false;
      }
      return;
    }
  });
}
