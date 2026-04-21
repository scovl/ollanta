export const ISSUE_PAGE = 50;

export const SEV_ORDER = ['blocker', 'critical', 'major', 'minor', 'info'];
export const SEV_COLOR = {
  blocker: '#ef4444',
  critical: '#f97316',
  major: '#eab308',
  minor: '#22c55e',
  info: '#64748b',
};
export const SEV_BG = {
  blocker: 'rgba(239,68,68,.12)',
  critical: 'rgba(249,115,22,.10)',
  major: 'rgba(234,179,8,.09)',
  minor: 'rgba(34,197,94,.09)',
  info: 'rgba(100,116,139,.09)',
};
export const SEV_LABEL = {
  blocker: 'Blocker',
  critical: 'Critical',
  major: 'Major',
  minor: 'Minor',
  info: 'Info',
};
export const TYPE_ICON = {
  bug: '\uD83D\uDC1B',
  code_smell: '\uD83C\uDF3F',
  vulnerability: '\uD83D\uDD12',
};
export const TYPE_COLOR = {
  bug: '#ef4444',
  code_smell: '#22c55e',
  vulnerability: '#f97316',
};
export const TYPE_LABEL = {
  bug: 'Bug',
  code_smell: 'Code Smell',
  vulnerability: 'Vulnerability',
};

export function emptyScope() {
  return { type: 'branch', branch: '', pullRequestKey: '', pullRequestBase: '', defaultBranch: '' };
}

export function createInitialState() {
  return {
    user: null,
    view: 'login',
    projects: [],
    currentProject: null,
    currentScan: null,
    scope: emptyScope(),
    overviewData: null,
    issues: [],
    issuesTotal: 0,
    issueOffset: 0,
    issueFilter: { severity: 'all', type: 'all', status: 'all', search: '' },
    loading: false,
    loadingIssues: false,
    projectTab: 'overview',
    gateData: null,
    webhooksData: null,
    profilesData: null,
    activityData: null,
    branchesData: null,
    pullRequestsData: null,
    projectInfoData: null,
    codeTreeData: null,
    codeFileData: null,
    codeSelectedPath: '',
    newCodePeriod: null,
    selectedIssue: null,
  };
}

export let state = createInitialState();

export function replaceState(nextState) {
  state = nextState;
  return state;
}

export function resetProjectState() {
  state.currentProject = null;
  state.currentScan = null;
  state.scope = emptyScope();
  state.overviewData = null;
  state.issues = [];
  state.issuesTotal = 0;
  state.issueOffset = 0;
  state.issueFilter = { severity: 'all', type: 'all', status: 'all', search: '' };
  state.projectTab = 'overview';
  state.gateData = null;
  state.webhooksData = null;
  state.profilesData = null;
  state.activityData = null;
  state.branchesData = null;
  state.pullRequestsData = null;
  state.projectInfoData = null;
  state.codeTreeData = null;
  state.codeFileData = null;
  state.codeSelectedPath = '';
  state.newCodePeriod = null;
  state.selectedIssue = null;
}