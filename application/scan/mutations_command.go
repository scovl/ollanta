package scan

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	mutationEnforcementEnforced   = "enforced"
	mutationEnforcementBestEffort = "best_effort"
	mutationEnforcementAdvisory   = "advisory"
)

type mutationCommandResult struct {
	Command     string
	Enforcement string
}

func buildMutationCommand(tool string, opts MutationOptions) mutationCommandResult {
	applyMutationDefaults(&opts)
	switch tool {
	case mutationToolStryker:
		return strykerMutationCommand(opts)
	case mutationToolPIT:
		return pitMutationCommand(opts)
	case mutationToolMutmut:
		return mutmutMutationCommand(opts)
	case mutationToolCosmic:
		return cosmicRayMutationCommand(opts)
	case mutationToolInfection:
		return infectionMutationCommand(opts)
	default:
		return mutationCommandResult{Enforcement: mutationEnforcementAdvisory}
	}
}

func strykerMutationCommand(opts MutationOptions) mutationCommandResult {
	args := []string{"npx", "stryker", "run"}
	enforcement := mutationEnforcementAdvisory
	if len(opts.ChangedFiles) > 0 {
		args = append(args, fmt.Sprintf("--mutate=%s", strings.Join(opts.ChangedFiles, ",")))
		enforcement = mutationEnforcementEnforced
	} else if opts.ChangedOnly {
		args = append(args, "--mutate")
		enforcement = mutationEnforcementBestEffort
	}
	if opts.MaxMutants > 0 {
		args = append(args, fmt.Sprintf("--concurrency=%d", minInt(opts.MaxMutants, 4)))
	}
	return mutationCommandResult{Command: strings.Join(args, " "), Enforcement: enforcement}
}

func pitMutationCommand(opts MutationOptions) mutationCommandResult {
	enforcement := mutationEnforcementAdvisory
	if fileExists("pom.xml") {
		cmd := "mvn test-compile org.pitest:pitest-maven:mutationCoverage"
		if opts.MaxMutants > 0 {
			cmd += fmt.Sprintf(" -DmaxMutationsPerClass=%d", opts.MaxMutants)
		}
		return mutationCommandResult{Command: cmd, Enforcement: enforcement}
	}
	return mutationCommandResult{Command: "./gradlew pitest", Enforcement: enforcement}
}

func mutmutMutationCommand(opts MutationOptions) mutationCommandResult {
	args := []string{"mutmut", "run"}
	enforcement := mutationEnforcementAdvisory
	if len(opts.ChangedFiles) > 0 {
		enforcement = mutationEnforcementEnforced
	} else if opts.ChangedOnly {
		enforcement = mutationEnforcementBestEffort
	}
	return mutationCommandResult{Command: strings.Join(args, " "), Enforcement: enforcement}
}

func cosmicRayMutationCommand(opts MutationOptions) mutationCommandResult {
	config := ".cosmic-ray.toml"
	args := []string{"cosmic-ray", "exec", config}
	enforcement := mutationEnforcementAdvisory
	if len(opts.ChangedFiles) > 0 {
		enforcement = mutationEnforcementEnforced
	} else if opts.ChangedOnly {
		enforcement = mutationEnforcementBestEffort
	}
	return mutationCommandResult{Command: strings.Join(args, " "), Enforcement: enforcement}
}

func infectionMutationCommand(opts MutationOptions) mutationCommandResult {
	args := []string{"vendor/bin/infection"}
	enforcement := mutationEnforcementAdvisory
	if len(opts.ChangedFiles) > 0 {
		args = append(args, fmt.Sprintf("--filter=%s", strings.Join(opts.ChangedFiles, ",")))
		args = append(args, "--git-diff-filter=AM")
		enforcement = mutationEnforcementEnforced
	} else if opts.ChangedOnly {
		args = append(args, "--git-diff-filter=AM")
		enforcement = mutationEnforcementBestEffort
	}
	if opts.MaxMutants > 0 {
		args = append(args, fmt.Sprintf("--threads=%d", minInt(opts.MaxMutants/10+1, 4)))
	}
	return mutationCommandResult{Command: strings.Join(args, " "), Enforcement: enforcement}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ResolveChangedFiles returns the list of files changed between the current
// branch and its base (or HEAD~1 when no base is available). An empty slice
// with a non-nil error means diff could not be computed.
func ResolveChangedFiles(projectDir, baseBranch string) ([]string, error) {
	if baseBranch == "" {
		return gitChangedFiles(projectDir, "HEAD~1")
	}
	return gitChangedFiles(projectDir, "origin/"+baseBranch, baseBranch)
}

func gitChangedFiles(projectDir string, baseRefs ...string) ([]string, error) {
	for _, baseRef := range baseRefs {
		args := []string{"diff", "--name-only", "--diff-filter=ACMR", baseRef, "HEAD"}
		if files, err := runGitDiff(projectDir, args); err == nil && len(files) > 0 {
			return files, nil
		}
	}
	return nil, nil
}

func runGitDiff(projectDir string, args []string) ([]string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	seen := map[string]bool{}
	for _, line := range lines {
		file := strings.TrimSpace(line)
		if file == "" || seen[file] {
			continue
		}
		seen[file] = true
		files = append(files, filepath.ToSlash(file))
	}
	return files, nil
}
