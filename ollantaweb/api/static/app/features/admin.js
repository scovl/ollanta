import { apiFetch } from '../core/api.js';
import { state } from '../core/state.js';
import { escAttr, escHtml } from '../core/utils.js';

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
  const ncpStr = ncp
    ? escHtml(ncp.strategy) + (ncp.value ? ' \u2014 ' + escHtml(ncp.value) : '')
    : 'auto (default)';

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

export function bindAdminTabContent() {
  const project = state.currentProject;
  if (!project) return;

  document.querySelectorAll('.expand-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + id);
      if (!box) return;
      const hidden = box.classList.toggle('hidden');
      btn.textContent = hidden ? 'Conditions \u25BE' : 'Conditions \u25B4';
      if (!hidden && box.innerHTML.includes('Loading')) {
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