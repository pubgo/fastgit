package gitconflict

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// File describes one conflicted path.
type File struct {
	Path   string
	Module string
	Reason string
}

// Snapshot is a structured conflict report.
type Snapshot struct {
	Files   []File
	Groups  map[string][]string
	Summary string
}

// ListFiles returns unmerged file paths in the repository.
func ListFiles(ctx context.Context, repoRoot string) ([]string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		var err error
		repoRoot, err = gitOutput(ctx, ".", "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
	}

	out, err := gitOutput(ctx, repoRoot, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	sort.Strings(files)
	return files, nil
}

// HasConflicts reports whether the repo has unmerged paths.
func HasConflicts(ctx context.Context, repoRoot string) bool {
	files, err := ListFiles(ctx, repoRoot)
	return err == nil && len(files) > 0
}

// BuildSnapshot creates a grouped conflict report.
func BuildSnapshot(ctx context.Context, repoRoot string) (Snapshot, error) {
	paths, err := ListFiles(ctx, repoRoot)
	if err != nil {
		return Snapshot{}, err
	}
	if len(paths) == 0 {
		return Snapshot{Summary: "No merge conflicts detected."}, nil
	}

	files := make([]File, 0, len(paths))
	groups := map[string][]string{}
	for _, path := range paths {
		module := moduleName(path)
		reason := suggestReason(path)
		files = append(files, File{Path: path, Module: module, Reason: reason})
		groups[module] = append(groups[module], path)
	}

	summary := renderSummary(files, groups)
	return Snapshot{Files: files, Groups: groups, Summary: summary}, nil
}

func moduleName(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return "(root)"
	}
	if len(parts) >= 2 && (parts[0] == "pkg" || parts[0] == "cmds" || parts[0] == "internal") {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func suggestReason(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".md"):
		return "Documentation conflict — verify wording and links."
	case strings.HasSuffix(lower, ".go"):
		return "Code conflict — inspect imports, signatures, and tests."
	case strings.Contains(lower, "go.mod") || strings.Contains(lower, "go.sum"):
		return "Dependency conflict — reconcile module versions."
	case strings.Contains(lower, "yaml") || strings.Contains(lower, "json"):
		return "Config conflict — compare keys and defaults."
	default:
		return "Review both sides and keep the intended behavior."
	}
}

func renderSummary(files []File, groups map[string][]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Detected %d conflicted file(s) in %d module group(s).\n\n", len(files), len(groups)))

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		b.WriteString(fmt.Sprintf("## %s\n", key))
		for _, path := range groups[key] {
			reason := ""
			for _, file := range files {
				if file.Path == path {
					reason = file.Reason
					break
				}
			}
			b.WriteString(fmt.Sprintf("- %s\n", path))
			if reason != "" {
				b.WriteString(fmt.Sprintf("  - %s\n", reason))
			}
		}
		b.WriteByte('\n')
	}

	b.WriteString("Next steps:\n")
	b.WriteString("1. Resolve files listed above\n")
	b.WriteString("2. Run `git add` on resolved files\n")
	b.WriteString("3. Continue rebase/merge (`git rebase --continue` or `git commit`)\n")
	b.WriteString("4. Optional: `fastgit conflict open` to edit files\n")
	return b.String()
}

func gitOutput(ctx context.Context, repoRoot string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", repoRoot}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
