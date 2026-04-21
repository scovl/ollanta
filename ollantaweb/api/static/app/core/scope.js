import { emptyScope, state } from './state.js';
import { escHtml } from './utils.js';

export function normalizeScope(scope) {
  const next = Object.assign(emptyScope(), scope);
  if (next.pull_request && !next.pullRequestKey) next.pullRequestKey = next.pull_request;
  if (next.pull_request_key && !next.pullRequestKey) next.pullRequestKey = next.pull_request_key;
  if (next.pull_request_base && !next.pullRequestBase) next.pullRequestBase = next.pull_request_base;
  if (next.default_branch && !next.defaultBranch) next.defaultBranch = next.default_branch;
  if (!next.type) next.type = next.pullRequestKey ? 'pull_request' : 'branch';
  if (next.type === 'pull_request') {
    next.branch = next.branch || '';
  } else {
    next.type = 'branch';
    next.pullRequestKey = '';
    next.pullRequestBase = '';
  }
  return next;
}

export function buildScopeQuery(scope) {
  const params = new URLSearchParams();
  const current = normalizeScope(scope || state.scope);
  if (current.type === 'pull_request' && current.pullRequestKey) {
    params.set('pull_request', current.pullRequestKey);
  } else if (current.branch) {
    params.set('branch', current.branch);
  }
  return params;
}

export function buildScopedPath(path, scope) {
  const params = buildScopeQuery(scope);
  const qs = params.toString();
  if (!qs) return path;
  return path + (path.includes('?') ? '&' : '?') + qs;
}

export function parseProjectRoute(search) {
  const params = new URLSearchParams(search || globalThis.location.search);
  return {
    project: params.get('project') || '',
    tab: params.get('tab') || 'overview',
    branch: params.get('branch') || '',
    pullRequest: params.get('pull_request') || '',
  };
}

export function buildProjectRoute(projectKey, tab, scope) {
  const params = new URLSearchParams();
  if (projectKey) params.set('project', projectKey);
  if (tab) params.set('tab', tab);
  const scopeParams = buildScopeQuery(scope);
  scopeParams.forEach((value, key) => params.set(key, value));
  const qs = params.toString();
  return qs ? '?' + qs : globalThis.location.pathname;
}

export function syncProjectUrl(replace) {
  if (!state.currentProject) return;
  const method = replace ? 'replaceState' : 'pushState';
  history[method]({}, '', buildProjectRoute(state.currentProject.key, state.projectTab, state.scope));
}

export function resolvedBranchName(scope) {
  const current = normalizeScope(scope || state.scope);
  return current.branch || current.defaultBranch || '';
}

export function activeScopeTitle(scope) {
  const current = normalizeScope(scope || state.scope);
  if (current.type === 'pull_request' && current.pullRequestKey) {
    return `Pull Request #${escHtml(current.pullRequestKey)}`;
  }
  return escHtml(resolvedBranchName(current) || 'Branch unavailable');
}

export function activeScopeSubtitle(scope) {
  const current = normalizeScope(scope || state.scope);
  if (current.type === 'pull_request' && current.pullRequestKey) {
    const branch = current.branch || 'source branch pending';
    const base = current.pullRequestBase || current.defaultBranch || 'base branch pending';
    return `${escHtml(branch)} \u2192 ${escHtml(base)}`;
  }
  const branch = resolvedBranchName(current);
  if (!branch) {
    return 'Git branch metadata unavailable';
  }
  if (current.defaultBranch && branch === current.defaultBranch) {
    return 'Default branch';
  }
  return 'Branch scope';
}