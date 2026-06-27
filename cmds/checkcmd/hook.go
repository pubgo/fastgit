package checkcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pubgo/redant"
)

type hookSpec struct {
	name   string
	script string
}

var managedHooks = []hookSpec{
	{
		name: "pre-commit",
		script: `#!/bin/sh
` + hookMarker + `
exec fastgit check run --staged-only
`,
	},
	{
		name: "pre-push",
		script: `#!/bin/sh
` + hookMarker + `
exec fastgit check run
`,
	},
}

func installHooks(inv *redant.Invocation, force bool) error {
	gitDir, err := resolveGitDir()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	for _, spec := range managedHooks {
		if err := installOneHook(inv, gitDir, spec, force); err != nil {
			return err
		}
	}
	return nil
}

func installOneHook(inv *redant.Invocation, gitDir string, spec hookSpec, force bool) error {
	hookPath := fmt.Sprintf("%s/hooks/%s", gitDir, spec.name)
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if strings.Contains(content, hookMarker) {
			_, _ = fmt.Fprintf(inv.Stdout, "%s hook already installed: %s\n", spec.name, hookPath)
			return nil
		}
		if !force {
			return fmt.Errorf("%s hook exists and is not fastgit-managed; use --force to overwrite", spec.name)
		}
	}

	if err := os.WriteFile(hookPath, []byte(spec.script), 0o755); err != nil {
		return fmt.Errorf("write %s hook: %w", spec.name, err)
	}
	_, _ = fmt.Fprintf(inv.Stdout, "installed %s hook: %s\n", spec.name, hookPath)
	return nil
}

func uninstallHooks(inv *redant.Invocation) error {
	gitDir, err := resolveGitDir()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	removed := 0
	for _, spec := range managedHooks {
		ok, err := removeOneHook(inv, gitDir, spec.name)
		if err != nil {
			return err
		}
		if ok {
			removed++
		}
	}
	if removed == 0 {
		_, _ = fmt.Fprintln(inv.Stdout, "no fastgit-managed hooks to remove")
	}
	return nil
}

func removeOneHook(inv *redant.Invocation, gitDir, name string) (bool, error) {
	hookPath := fmt.Sprintf("%s/hooks/%s", gitDir, name)
	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !strings.Contains(string(data), hookMarker) {
		return false, nil
	}
	if err := os.Remove(hookPath); err != nil {
		return false, fmt.Errorf("remove %s hook: %w", name, err)
	}
	_, _ = fmt.Fprintf(inv.Stdout, "removed %s hook: %s\n", name, hookPath)
	return true, nil
}

func resolveGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cwd, dir)
	}
	return dir, nil
}
