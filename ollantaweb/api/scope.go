package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

const projectNotFoundMessage = "project not found"

type scopeResponse struct {
	Type            string `json:"type"`
	Branch          string `json:"branch,omitempty"`
	PullRequestKey  string `json:"pull_request_key,omitempty"`
	PullRequestBase string `json:"pull_request_base,omitempty"`
	DefaultBranch   string `json:"default_branch,omitempty"`
}

type resolvedProjectScope struct {
	Project       *postgres.Project
	Scope         model.AnalysisScope
	DefaultBranch string
}

func parseScopeQuery(r *http.Request) (model.AnalysisScope, error) {
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	pullRequest := strings.TrimSpace(r.URL.Query().Get("pull_request"))
	if branch != "" && pullRequest != "" {
		return model.AnalysisScope{}, fmt.Errorf("branch and pull_request are mutually exclusive")
	}
	if pullRequest != "" {
		return model.AnalysisScope{Type: model.ScopeTypePullRequest, PullRequestKey: pullRequest}, nil
	}
	return model.AnalysisScope{Type: model.ScopeTypeBranch, Branch: branch}, nil
}

func resolveProjectScope(ctx context.Context, projects *postgres.ProjectRepository, scans *postgres.ScanRepository, key string, requested model.AnalysisScope) (*resolvedProjectScope, error) {
	project, err := projects.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return resolveProjectScopeForProject(ctx, project, scans, requested)
}

func resolveProjectScopeForProject(ctx context.Context, project *postgres.Project, scans *postgres.ScanRepository, requested model.AnalysisScope) (*resolvedProjectScope, error) {
	if project == nil {
		return nil, errors.New("project is required")
	}
	scope := requested.Normalize()
	defaultBranch, _, err := scans.ResolveDefaultBranch(ctx, project.ID, project.MainBranch)
	if err != nil {
		return nil, err
	}
	if scope.Type != model.ScopeTypePullRequest && scope.Branch == "" {
		scope.Branch = defaultBranch
	}
	return &resolvedProjectScope{Project: project, Scope: scope, DefaultBranch: defaultBranch}, nil
}

func toScopeResponse(resolved *resolvedProjectScope) *scopeResponse {
	if resolved == nil {
		return nil
	}
	return &scopeResponse{
		Type:            resolved.Scope.Normalize().Type,
		Branch:          resolved.Scope.Branch,
		PullRequestKey:  resolved.Scope.PullRequestKey,
		PullRequestBase: resolved.Scope.PullRequestBase,
		DefaultBranch:   resolved.DefaultBranch,
	}
}
