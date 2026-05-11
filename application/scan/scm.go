package scan

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/scovl/ollanta/domain/model"
)

type envLookup func(string) (string, bool)
type gitRunner func(string, ...string) (string, error)

// SCMContext is the resolved source-control metadata for a scan run.
type SCMContext struct {
	ScopeType       string
	Branch          string
	CommitSHA       string
	PullRequestKey  string
	PullRequestBase string
}

type pullRequestMetadata struct {
	Key    string
	Branch string
	Base   string
}

var githubPRRefPattern = regexp.MustCompile(`refs/pull/(\d+)/`)

func resolveSCMContext(opts *ScanOptions) (*SCMContext, error) {
	return resolveSCMContextWithInputs(opts, os.LookupEnv, runGit)
}

func resolveSCMContextWithInputs(opts *ScanOptions, lookup envLookup, git gitRunner) (*SCMContext, error) {
	ctx := &SCMContext{ScopeType: model.ScopeTypeBranch}
	pr := mergePullRequestMetadata(resolvePullRequestEnv(lookup), pullRequestMetadata{
		Key:    opts.PullRequestKey,
		Branch: opts.PullRequestBranch,
		Base:   opts.PullRequestBase,
	})

	if missing := pr.missingFields(); len(missing) > 0 {
		return nil, fmt.Errorf("incomplete pull request metadata: missing %s", strings.Join(missing, ", "))
	}
	if pr.Key != "" {
		ctx.ScopeType = model.ScopeTypePullRequest
		ctx.PullRequestKey = pr.Key
		ctx.PullRequestBase = pr.Base
		ctx.Branch = pr.Branch
	}

	if strings.TrimSpace(opts.Branch) != "" {
		ctx.Branch = strings.TrimSpace(opts.Branch)
	}
	if strings.TrimSpace(opts.CommitSHA) != "" {
		ctx.CommitSHA = strings.TrimSpace(opts.CommitSHA)
	}

	gitBranch, gitCommit, detached, insideRepo, err := detectGitContext(opts.ProjectDir, git)
	if err != nil {
		return nil, err
	}

	if ctx.CommitSHA == "" {
		ctx.CommitSHA = gitCommit
	}
	if ctx.Branch == "" {
		ctx.Branch = gitBranch
	}

	if insideRepo && detached && ctx.Branch == "" {
		return nil, fmt.Errorf("detached HEAD detected; provide -branch explicitly")
	}

	return ctx, nil
}

func resolvePullRequestEnv(lookup envLookup) pullRequestMetadata {
	sources := []pullRequestMetadata{
		customPullRequestMetadata(lookup),
		githubPullRequestMetadata(lookup),
		gitLabPullRequestMetadata(lookup),
		azurePullRequestMetadata(lookup),
	}
	for _, source := range sources {
		if source.Key != "" || source.Branch != "" || source.Base != "" {
			return source
		}
	}
	return pullRequestMetadata{}
}

func customPullRequestMetadata(lookup envLookup) pullRequestMetadata {
	return pullRequestMetadata{
		Key:    envValue(lookup, "OLLANTA_PULL_REQUEST_KEY"),
		Branch: envValue(lookup, "OLLANTA_PULL_REQUEST_BRANCH"),
		Base:   envValue(lookup, "OLLANTA_PULL_REQUEST_BASE"),
	}
}

func githubPullRequestMetadata(lookup envLookup) pullRequestMetadata {
	key := ""
	if ref := envValue(lookup, "GITHUB_REF"); ref != "" {
		if match := githubPRRefPattern.FindStringSubmatch(ref); len(match) == 2 {
			key = match[1]
		}
	}
	return pullRequestMetadata{
		Key:    key,
		Branch: envValue(lookup, "GITHUB_HEAD_REF"),
		Base:   envValue(lookup, "GITHUB_BASE_REF"),
	}
}

func gitLabPullRequestMetadata(lookup envLookup) pullRequestMetadata {
	return pullRequestMetadata{
		Key:    envValue(lookup, "CI_MERGE_REQUEST_IID"),
		Branch: envValue(lookup, "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"),
		Base:   envValue(lookup, "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"),
	}
}

func azurePullRequestMetadata(lookup envLookup) pullRequestMetadata {
	return pullRequestMetadata{
		Key:    envValue(lookup, "SYSTEM_PULLREQUEST_PULLREQUESTNUMBER"),
		Branch: trimAzureRef(envValue(lookup, "SYSTEM_PULLREQUEST_SOURCEBRANCH")),
		Base:   trimAzureRef(envValue(lookup, "SYSTEM_PULLREQUEST_TARGETBRANCH")),
	}
}

func mergePullRequestMetadata(base, override pullRequestMetadata) pullRequestMetadata {
	if override.Key != "" {
		base.Key = override.Key
	}
	if override.Branch != "" {
		base.Branch = override.Branch
	}
	if override.Base != "" {
		base.Base = override.Base
	}
	return base
}

func (m pullRequestMetadata) missingFields() []string {
	if m.Key == "" && m.Branch == "" && m.Base == "" {
		return nil
	}
	missing := make([]string, 0, 3)
	if m.Key == "" {
		missing = append(missing, "pull_request_key")
	}
	if m.Branch == "" {
		missing = append(missing, "pull_request_branch")
	}
	if m.Base == "" {
		missing = append(missing, "pull_request_base")
	}
	sort.Strings(missing)
	return missing
}

func detectGitContext(projectDir string, git gitRunner) (branch, commit string, detached, insideRepo bool, err error) {
	inside, err := git(projectDir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return "", "", false, false, nil
		}
		return "", "", false, false, nil
	}
	if strings.TrimSpace(inside) != "true" {
		return "", "", false, false, nil
	}
	insideRepo = true

	commit, err = git(projectDir, "rev-parse", "HEAD")
	if err != nil {
		return "", "", false, true, fmt.Errorf("detect git commit: %w", err)
	}

	branch, err = git(projectDir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", strings.TrimSpace(commit), true, true, nil
	}

	return strings.TrimSpace(branch), strings.TrimSpace(commit), false, true, nil
}

func runGit(projectDir string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	cmdArgs := append([]string{"-C", projectDir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func envValue(lookup envLookup, key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func trimAzureRef(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "refs/heads/") {
		return strings.TrimPrefix(value, "refs/heads/")
	}
	return value
}
