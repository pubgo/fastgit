package prcmd

import (
	"context"
	"fmt"
	"strings"
)

// Draft holds generated PR metadata before submission.
type Draft struct {
	Title string
	Body  string
	Base  string
	Head  string
}

// BuildDraft assembles a PR title and body from git history (rule-based, no AI).
func BuildDraft(ctx context.Context, rc RepoContext) (Draft, error) {
	commits, err := gitOutput(ctx, rc.RepoRoot, "log", fmt.Sprintf("%s..HEAD", rc.BaseRef), "--pretty=format:- %s (%an)", "--reverse")
	if err != nil {
		return Draft{}, err
	}

	diffStat, err := gitOutput(ctx, rc.RepoRoot, "diff", rc.BaseRef+"..HEAD", "--stat")
	if err != nil {
		return Draft{}, err
	}
	diffNames, err := gitOutput(ctx, rc.RepoRoot, "diff", rc.BaseRef+"..HEAD", "--name-only")
	if err != nil {
		return Draft{}, err
	}

	title := suggestTitle(rc.Branch, commits)
	body := renderBody(commits, diffStat, diffNames, rc)

	return Draft{
		Title: title,
		Body:  body,
		Base:  baseBranchName(rc.BaseRef),
		Head:  rc.Branch,
	}, nil
}

func suggestTitle(branch, commits string) string {
	lines := strings.Split(strings.TrimSpace(commits), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		first := strings.TrimSpace(lines[0])
		first = strings.TrimPrefix(first, "- ")
		if idx := strings.Index(first, " ("); idx > 0 {
			first = first[:idx]
		}
		if len(first) > 72 {
			first = first[:69] + "..."
		}
		if first != "" {
			return first
		}
	}

	title := strings.ReplaceAll(branch, "/", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		return "Update branch"
	}
	return title
}

func renderBody(commits, diffStat, diffNames string, rc RepoContext) string {
	var b strings.Builder
	b.WriteString("## Summary\n\n")
	if strings.TrimSpace(commits) == "" {
		b.WriteString("- No commits ahead of base (PR may only contain unpushed commits)\n")
	} else {
		b.WriteString(commits)
		b.WriteByte('\n')
	}

	b.WriteString("\n## Changed files\n\n")
	if strings.TrimSpace(diffNames) == "" {
		b.WriteString("_No file changes detected against base._\n")
	} else {
		b.WriteString("```\n")
		b.WriteString(strings.TrimSpace(diffStat))
		b.WriteString("\n```\n")
	}

	b.WriteString("\n## Risk\n\n")
	for _, line := range assessRisk(diffNames) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteByte('\n')
	}

	b.WriteString("\n## Test plan\n\n")
	b.WriteString("- [ ] Run `fastgit check run`\n")
	b.WriteString("- [ ] Verify affected modules manually\n")

	b.WriteString("\n## Rollback\n\n")
	b.WriteString(fmt.Sprintf("- Revert commits on `%s` or close PR without merge\n", rc.Branch))
	b.WriteString(fmt.Sprintf("- Base branch: `%s`\n", rc.BaseRef))

	return b.String()
}

func assessRisk(diffNames string) []string {
	names := strings.Split(strings.TrimSpace(diffNames), "\n")
	if len(names) == 0 || (len(names) == 1 && names[0] == "") {
		return []string{"Low: no diff against base detected"}
	}

	risks := make([]string, 0, 4)
	if len(names) >= 20 {
		risks = append(risks, fmt.Sprintf("Medium: large change set (%d files)", len(names)))
	}

	sensitive := []string{".env", "secret", "credential", "auth", "migration", "deploy"}
	for _, name := range names {
		lower := strings.ToLower(name)
		for _, token := range sensitive {
			if strings.Contains(lower, token) {
				risks = append(risks, fmt.Sprintf("Review carefully: touches %q", name))
				break
			}
		}
	}

	if len(risks) == 0 {
		risks = append(risks, "Low: no obvious high-risk paths detected (heuristic)")
	}
	return risks
}
