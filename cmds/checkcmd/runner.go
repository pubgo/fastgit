package checkcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunOptions controls check execution.
type RunOptions struct {
	StagedOnly bool
	Fix        bool
	DryRun     bool
	RepoRoot   string
}

// StepResult holds the outcome of one step.
type StepResult struct {
	Step    Step
	Skipped bool
	Reason  string
	Err     error
	Output  string
}

// Run executes the check pipeline.
func Run(ctx context.Context, cfg Config, opts RunOptions) ([]StepResult, error) {
	stagedFiles, err := listStagedFiles(opts.RepoRoot)
	if err != nil {
		return nil, err
	}

	if opts.StagedOnly && len(stagedFiles) == 0 {
		return nil, fmt.Errorf("no staged files; stage changes or omit --staged-only")
	}

	var results []StepResult
	for _, step := range cfg.Steps {
		result := runStep(ctx, step, opts, stagedFiles)
		results = append(results, result)
		if result.Err != nil && !result.Skipped {
			return results, result.Err
		}
	}
	return results, nil
}

func runStep(ctx context.Context, step Step, opts RunOptions, stagedFiles []string) StepResult {
	result := StepResult{Step: step}

	if step.Name == "secrets" {
		cmdStr, tool := resolveSecretScanCommand(opts.StagedOnly)
		if cmdStr == "" {
			result.Skipped = true
			result.Reason = "install gitleaks or trufflehog to enable secret scan"
			if opts.DryRun {
				result.Output = fmt.Sprintf("[dry-run] would skip %s: %s", step.Name, result.Reason)
			}
			return result
		}
		if opts.DryRun {
			result.Output = fmt.Sprintf("[dry-run] %s (%s): %s", step.Name, tool, cmdStr)
			return result
		}
		output, err := execInDir(ctx, opts.RepoRoot, cmdStr)
		result.Output = strings.TrimSpace(output)
		if err != nil {
			result.Err = fmt.Errorf("%s failed: %w\n%s", step.Name, err, result.Output)
		}
		return result
	}

	if step.Optional && !commandExists(firstToken(step.Command)) {
		result.Skipped = true
		result.Reason = fmt.Sprintf("%s not found in PATH", firstToken(step.Command))
		if opts.DryRun {
			result.Output = fmt.Sprintf("[dry-run] would skip %s: %s", step.Name, result.Reason)
		}
		return result
	}

	cmdStr := step.Command
	if opts.Fix && step.Fixable && strings.TrimSpace(step.FixCommand) != "" {
		cmdStr = step.FixCommand
	}

	if opts.StagedOnly && step.Name == "fmt" {
		cmdStr = stagedFmtCommand(stagedFiles, opts.Fix && step.Fixable)
		if cmdStr == "" {
			result.Skipped = true
			result.Reason = "no staged Go files"
			return result
		}
	}

	if opts.DryRun {
		result.Output = fmt.Sprintf("[dry-run] %s: %s", step.Name, cmdStr)
		return result
	}

	output, err := execInDir(ctx, opts.RepoRoot, cmdStr)
	result.Output = strings.TrimSpace(output)
	if err != nil {
		result.Err = fmt.Errorf("%s failed: %w\n%s", step.Name, err, result.Output)
		return result
	}

	if step.Name == "fmt" && !opts.Fix && strings.TrimSpace(result.Output) != "" {
		result.Err = fmt.Errorf("fmt failed: unformatted files:\n%s", result.Output)
	}
	return result
}

func stagedFmtCommand(stagedFiles []string, fix bool) string {
	var goFiles []string
	for _, f := range stagedFiles {
		if strings.HasSuffix(f, ".go") {
			goFiles = append(goFiles, f)
		}
	}
	if len(goFiles) == 0 {
		return ""
	}
	flag := "-l"
	if fix {
		flag = "-w"
	}
	return "gofmt " + flag + " " + strings.Join(goFiles, " ")
}

func listStagedFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list staged files: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func execInDir(ctx context.Context, dir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func commandExists(name string) bool {
	if name == "" {
		return false
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func firstToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
