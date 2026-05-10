package scan

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

const (
	gitInsideWorkTreeCmd = "rev-parse --is-inside-work-tree"
	gitHeadCmd           = "rev-parse HEAD"
	gitBranchCmd         = "symbolic-ref --quiet --short HEAD"
	unexpectedGitCommand = "unexpected git command"
	resolvedContextError = "resolveSCMContextWithInputs() error = %v"
	scopeTypeMismatch    = "ScopeType = %q, want %q"
	resolvedCommitSHA    = "abcdef123456"
	commitSHAMismatch    = "CommitSHA = %q, want abcdef123456"
)

func TestResolveSCMContextWithInputsUsesPullRequestEnvironment(t *testing.T) {
	t.Parallel()

	ctx, err := resolveSCMContextWithInputs(
		&ScanOptions{ProjectDir: "."},
		pullRequestLookup(),
		gitCommandMock(),
	)
	if err != nil {
		t.Fatalf(resolvedContextError, err)
	}
	assertPRContext(t, ctx)
}

func pullRequestLookup() func(string) (string, bool) {
	return func(key string) (string, bool) {
		values := map[string]string{
			"OLLANTA_PULL_REQUEST_KEY":    "42",
			"OLLANTA_PULL_REQUEST_BRANCH": "feature/login",
			"OLLANTA_PULL_REQUEST_BASE":   "main",
		}
		value, ok := values[key]
		return value, ok
	}
}

func gitCommandMock() func(string, ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case gitInsideWorkTreeCmd:
			return "true", nil
		case gitHeadCmd:
			return resolvedCommitSHA, nil
		case gitBranchCmd:
			return "ignored-branch", nil
		default:
			return "", errors.New(unexpectedGitCommand)
		}
	}
}

func assertPRContext(t *testing.T, ctx *SCMContext) {
	t.Helper()
	if ctx.ScopeType != model.ScopeTypePullRequest {
		t.Fatalf(scopeTypeMismatch, ctx.ScopeType, model.ScopeTypePullRequest)
	}
	if ctx.PullRequestKey != "42" {
		t.Fatalf("PullRequestKey = %q, want 42", ctx.PullRequestKey)
	}
	if ctx.Branch != "feature/login" {
		t.Fatalf("Branch = %q, want feature/login", ctx.Branch)
	}
	if ctx.PullRequestBase != "main" {
		t.Fatalf("PullRequestBase = %q, want main", ctx.PullRequestBase)
	}
	if ctx.CommitSHA != resolvedCommitSHA {
		t.Fatalf(commitSHAMismatch, ctx.CommitSHA)
	}
}

func TestResolveSCMContextWithInputsRejectsIncompletePullRequestMetadata(t *testing.T) {
	t.Parallel()

	lookup := func(key string) (string, bool) {
		if key == "OLLANTA_PULL_REQUEST_KEY" {
			return "42", true
		}
		return "", false
	}

	_, err := resolveSCMContextWithInputs(&ScanOptions{ProjectDir: "."}, lookup, func(string, ...string) (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("expected error for incomplete pull request metadata")
	}
	if !strings.Contains(err.Error(), "pull_request_base") || !strings.Contains(err.Error(), "pull_request_branch") {
		t.Fatalf("error = %q, want missing branch and base metadata", err)
	}
}

func TestResolveSCMContextWithInputsUsesExplicitBranchOverride(t *testing.T) {
	t.Parallel()

	git := func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case gitInsideWorkTreeCmd:
			return "true", nil
		case gitHeadCmd:
			return resolvedCommitSHA, nil
		case gitBranchCmd:
			return "release", nil
		default:
			return "", errors.New(unexpectedGitCommand)
		}
	}

	ctx, err := resolveSCMContextWithInputs(&ScanOptions{ProjectDir: ".", Branch: "hotfix"}, func(string) (string, bool) { return "", false }, git)
	if err != nil {
		t.Fatalf(resolvedContextError, err)
	}
	if ctx.Branch != "hotfix" {
		t.Fatalf("Branch = %q, want hotfix", ctx.Branch)
	}
	if ctx.CommitSHA != resolvedCommitSHA {
		t.Fatalf(commitSHAMismatch, ctx.CommitSHA)
	}
}

func TestResolveSCMContextWithInputsAllowsNonGitDirectories(t *testing.T) {
	t.Parallel()

	ctx, err := resolveSCMContextWithInputs(&ScanOptions{ProjectDir: "."}, func(string) (string, bool) { return "", false }, func(string, ...string) (string, error) {
		return "", &exec.Error{Name: "git", Err: exec.ErrNotFound}
	})
	if err != nil {
		t.Fatalf(resolvedContextError, err)
	}
	if ctx.ScopeType != model.ScopeTypeBranch {
		t.Fatalf(scopeTypeMismatch, ctx.ScopeType, model.ScopeTypeBranch)
	}
	if ctx.Branch != "" {
		t.Fatalf("Branch = %q, want empty for non-git directory", ctx.Branch)
	}
	if ctx.CommitSHA != "" {
		t.Fatalf("CommitSHA = %q, want empty for non-git directory", ctx.CommitSHA)
	}
}

func TestResolveSCMContextWithInputsRejectsDetachedHeadWithoutBranch(t *testing.T) {
	t.Parallel()

	git := func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case gitInsideWorkTreeCmd:
			return "true", nil
		case gitHeadCmd:
			return resolvedCommitSHA, nil
		case gitBranchCmd:
			return "", errors.New("detached")
		default:
			return "", errors.New(unexpectedGitCommand)
		}
	}

	_, err := resolveSCMContextWithInputs(&ScanOptions{ProjectDir: "."}, func(string) (string, bool) { return "", false }, git)
	if err == nil {
		t.Fatal("expected detached HEAD error")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("error = %q, want detached HEAD guidance", err)
	}
}

func TestResolveSCMContextWithInputsAllowsDetachedHeadWithExplicitBranch(t *testing.T) {
	t.Parallel()

	git := func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case gitInsideWorkTreeCmd:
			return "true", nil
		case gitHeadCmd:
			return resolvedCommitSHA, nil
		case gitBranchCmd:
			return "", errors.New("detached")
		default:
			return "", errors.New(unexpectedGitCommand)
		}
	}

	ctx, err := resolveSCMContextWithInputs(&ScanOptions{ProjectDir: ".", Branch: "release/1.2"}, func(string) (string, bool) { return "", false }, git)
	if err != nil {
		t.Fatalf(resolvedContextError, err)
	}
	if ctx.ScopeType != model.ScopeTypeBranch {
		t.Fatalf(scopeTypeMismatch, ctx.ScopeType, model.ScopeTypeBranch)
	}
	if ctx.Branch != "release/1.2" {
		t.Fatalf("Branch = %q, want release/1.2", ctx.Branch)
	}
	if ctx.CommitSHA != resolvedCommitSHA {
		t.Fatalf(commitSHAMismatch, ctx.CommitSHA)
	}
}
