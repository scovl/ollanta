'use strict';

import { configureAdminFeature } from './features/admin.js';
import { configureActivityFeature } from './features/activity.js';
import { configureCodeFeature } from './features/code.js';
import { configureIssuesFeature } from './features/issues.js';
import { configureProjectInformationFeature } from './features/project-information.js';
import { configureProjectFlowFeature } from './project-flow.js';
import { render, showToast } from './shell.js';

configureProjectFlowFeature({ render });
configureActivityFeature({ render });
configureCodeFeature({ render });
configureProjectInformationFeature({ render });
configureAdminFeature({ render, showToast });
configureIssuesFeature({ showToast });

export { createInitialState, replaceState, state } from './core/state.js';
export { buildProjectRoute, buildScopeQuery, normalizeScope, parseProjectRoute } from './core/scope.js';
export { loadCodeTreeData, renderCodeTab } from './features/code.js';
export { renderOverviewTab } from './features/overview.js';
export { renderProjectInformationTab } from './features/project-information.js';
export { changeScope, loadProject, renderScopeToolbar, switchTab } from './project-flow.js';
export { bootBrowserApp, init, loadProjects, render } from './shell.js';
