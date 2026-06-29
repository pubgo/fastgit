package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommandResult struct {
	Command   string `json:"command"`
	Output    string `json:"output"`
	ExitCode  int    `json:"exitCode"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

type FastgitService struct {
	repoRoot string
}

func NewFastgitService() *FastgitService {
	cwd, _ := os.Getwd()
	if filepath.Base(cwd) == "desktop" {
		parent := filepath.Dir(cwd)
		if st, err := os.Stat(filepath.Join(parent, "main.go")); err == nil && !st.IsDir() {
			cwd = parent
		}
	}
	return &FastgitService{repoRoot: cwd}
}

func (s *FastgitService) GetRepoRoot() string {
	return s.repoRoot
}

func (s *FastgitService) SetRepoRoot(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("repo path cannot be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", abs)
	}
	s.repoRoot = abs
	return nil
}

func (s *FastgitService) RunFastgit(commandLine string) (CommandResult, error) {
	line := strings.TrimSpace(commandLine)
	if line == "" {
		return CommandResult{}, errors.New("command cannot be empty")
	}
	args := strings.Fields(line)
	if len(args) == 0 {
		return CommandResult{}, errors.New("invalid command")
	}

	now := time.Now().UTC()
	result := CommandResult{Command: "fastgit " + line, StartedAt: now.Format(time.RFC3339)}

	bin, binArgs, err := s.resolveFastgitInvocation(args)
	if err != nil {
		return result, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, binArgs...)
	cmd.Dir = s.repoRoot
	output, runErr := cmd.CombinedOutput()

	result.Output = strings.TrimSpace(string(output))
	result.EndedAt = time.Now().UTC().Format(time.RFC3339)

	if runErr == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		if result.Output == "" {
			result.Output = runErr.Error()
		}
		return result, nil
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.ExitCode = 124
		if result.Output == "" {
			result.Output = "command timed out after 2m"
		}
		return result, nil
	}

	return result, runErr
}

func (s *FastgitService) resolveFastgitInvocation(args []string) (string, []string, error) {
	if path, err := exec.LookPath("fastgit"); err == nil {
		return path, args, nil
	}

	if path, err := exec.LookPath("go"); err == nil {
		fallback := []string{"run", "."}
		fallback = append(fallback, args...)
		return path, fallback, nil
	}

	return "", nil, errors.New("cannot find fastgit binary or go toolchain in PATH")
}
