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

type ActionField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
}

type ModuleAction struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Tool        string        `json:"tool"`
	Args        []string      `json:"args"`
	Fields      []ActionField `json:"fields"`
}

type DesktopModule struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Actions     []ModuleAction `json:"actions"`
}

type ActionRunRequest struct {
	ModuleID string            `json:"moduleID"`
	ActionID string            `json:"actionID"`
	Values   map[string]string `json:"values"`
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

func (s *FastgitService) GetModules() []DesktopModule {
	return []DesktopModule{
		{
			ID:          "repo",
			Title:       "仓库管理",
			Description: "查看状态、拉取、推送",
			Actions: []ModuleAction{
				{ID: "repo_status", Title: "状态", Description: "ggc status short", Tool: "fastgit", Args: []string{"ggc", "status", "short"}},
				{ID: "repo_pull", Title: "拉取", Description: "pull current branch", Tool: "fastgit", Args: []string{"pull"}},
				{ID: "repo_push", Title: "推送", Description: "push current branch", Tool: "fastgit", Args: []string{"push"}},
			},
		},
		{
			ID:          "branch",
			Title:       "分支管理",
			Description: "列举、创建、切换、删除分支",
			Actions: []ModuleAction{
				{ID: "branch_list", Title: "列出本地分支", Description: "ggc branch list local", Tool: "fastgit", Args: []string{"ggc", "branch", "list", "local"}},
				{ID: "branch_create", Title: "创建分支", Description: "ggc branch create <name>", Tool: "fastgit", Args: []string{"ggc", "branch", "create", "{{name}}"}, Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "feature/my-branch", Required: true}}},
				{ID: "branch_checkout", Title: "切换分支", Description: "ggc branch checkout <name>", Tool: "fastgit", Args: []string{"ggc", "branch", "checkout", "{{name}}"}, Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "main", Required: true}}},
				{ID: "branch_delete", Title: "删除分支", Description: "ggc branch delete <name>", Tool: "fastgit", Args: []string{"ggc", "branch", "delete", "{{name}}"}, Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "feature/my-branch", Required: true}}},
			},
		},
		{
			ID:          "worktree",
			Title:       "Worktree 管理",
			Description: "列出、创建、删除 worktree",
			Actions: []ModuleAction{
				{ID: "worktree_list", Title: "列出 worktree", Description: "worktree list", Tool: "fastgit", Args: []string{"worktree", "list"}},
				{ID: "worktree_create", Title: "创建 worktree", Description: "worktree create <issue|branch> --base <branch>", Tool: "fastgit", Args: []string{"worktree", "create", "--base", "{{base}}", "{{target}}"}, Fields: []ActionField{{Key: "target", Label: "issue/branch", Placeholder: "123 或 feature/abc", Required: true}, {Key: "base", Label: "base", Placeholder: "main", Required: true, Default: "main"}}},
				{ID: "worktree_remove", Title: "删除 worktree", Description: "worktree remove <issue|branch>", Tool: "fastgit", Args: []string{"worktree", "remove", "{{target}}"}, Fields: []ActionField{{Key: "target", Label: "issue/branch", Placeholder: "123 或 feature/abc", Required: true}}},
			},
		},
		{
			ID:          "issue",
			Title:       "Issue 管理",
			Description: "通过 gh CLI 进行 issue 查询与创建",
			Actions: []ModuleAction{
				{ID: "issue_list", Title: "列出 issue", Description: "gh issue list --limit 30", Tool: "gh", Args: []string{"issue", "list", "--limit", "30"}},
				{ID: "issue_view", Title: "查看 issue", Description: "gh issue view <id>", Tool: "gh", Args: []string{"issue", "view", "{{id}}"}, Fields: []ActionField{{Key: "id", Label: "Issue ID", Placeholder: "123", Required: true}}},
				{ID: "issue_create", Title: "创建 issue", Description: "gh issue create", Tool: "gh", Args: []string{"issue", "create", "--title", "{{title}}", "--body", "{{body}}"}, Fields: []ActionField{{Key: "title", Label: "标题", Placeholder: "Issue title", Required: true}, {Key: "body", Label: "正文", Placeholder: "Issue body", Required: true}}},
			},
		},
		{
			ID:          "pr",
			Title:       "PR 管理",
			Description: "create/status/sync/merge",
			Actions: []ModuleAction{
				{ID: "pr_status", Title: "PR 状态", Description: "fastgit pr status", Tool: "fastgit", Args: []string{"pr", "status"}},
				{ID: "pr_create", Title: "创建 PR", Description: "fastgit pr create", Tool: "fastgit", Args: []string{"pr", "create"}},
				{ID: "pr_sync", Title: "同步 PR", Description: "fastgit pr sync", Tool: "fastgit", Args: []string{"pr", "sync"}},
				{ID: "pr_merge", Title: "合并 PR", Description: "fastgit pr merge --method <method> --yes", Tool: "fastgit", Args: []string{"pr", "merge", "--method", "{{method}}", "--yes"}, Fields: []ActionField{{Key: "method", Label: "merge method", Placeholder: "squash|merge|rebase", Required: true, Default: "squash"}}},
			},
		},
		{
			ID:          "tag",
			Title:       "Tag 管理",
			Description: "查看和发布 tag",
			Actions: []ModuleAction{
				{ID: "tag_list", Title: "列出 tag", Description: "git tag --sort=-committerdate", Tool: "git", Args: []string{"tag", "--sort=-committerdate"}},
				{ID: "tag_publish", Title: "发布 tag", Description: "git tag <name> && git push origin <name>", Tool: "git", Args: []string{"tag", "{{name}}"}, Fields: []ActionField{{Key: "name", Label: "Tag", Placeholder: "v1.2.3", Required: true}}},
				{ID: "tag_push", Title: "推送 tag", Description: "git push origin <name>", Tool: "git", Args: []string{"push", "origin", "{{name}}"}, Fields: []ActionField{{Key: "name", Label: "Tag", Placeholder: "v1.2.3", Required: true}}},
			},
		},
	}
}

func (s *FastgitService) RunAction(req ActionRunRequest) (CommandResult, error) {
	module, action, err := s.findAction(req.ModuleID, req.ActionID)
	if err != nil {
		return CommandResult{}, err
	}
	_ = module
	values := req.Values
	if values == nil {
		values = map[string]string{}
	}

	expandedArgs, err := fillActionArgs(action, values)
	if err != nil {
		return CommandResult{}, err
	}

	return s.runTool(action.Tool, expandedArgs)
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
	return s.runTool("fastgit", args)
}

func fillActionArgs(action ModuleAction, values map[string]string) ([]string, error) {
	normalized := make(map[string]string, len(values))
	for k, v := range values {
		normalized[k] = strings.TrimSpace(v)
	}

	for _, field := range action.Fields {
		val := normalized[field.Key]
		if val == "" && field.Default != "" {
			val = field.Default
			normalized[field.Key] = val
		}
		if field.Required && val == "" {
			return nil, fmt.Errorf("missing required field: %s", field.Label)
		}
	}

	expanded := make([]string, 0, len(action.Args))
	for _, item := range action.Args {
		replaced := item
		for key, value := range normalized {
			replaced = strings.ReplaceAll(replaced, "{{"+key+"}}", value)
		}
		if strings.Contains(replaced, "{{") {
			return nil, fmt.Errorf("unresolved parameter in args: %s", item)
		}
		replaced = strings.TrimSpace(replaced)
		if replaced != "" {
			expanded = append(expanded, replaced)
		}
	}

	return expanded, nil
}

func (s *FastgitService) findAction(moduleID, actionID string) (DesktopModule, ModuleAction, error) {
	for _, module := range s.GetModules() {
		if module.ID != moduleID {
			continue
		}
		for _, action := range module.Actions {
			if action.ID == actionID {
				return module, action, nil
			}
		}
		return DesktopModule{}, ModuleAction{}, fmt.Errorf("action not found: %s", actionID)
	}
	return DesktopModule{}, ModuleAction{}, fmt.Errorf("module not found: %s", moduleID)
}

func (s *FastgitService) runTool(tool string, args []string) (CommandResult, error) {
	now := time.Now().UTC()
	result := CommandResult{StartedAt: now.Format(time.RFC3339)}

	bin, execArgs, err := s.resolveInvocation(tool, args)
	if err != nil {
		return result, err
	}
	result.Command = strings.Join(append([]string{bin}, execArgs...), " ")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, execArgs...)
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

func (s *FastgitService) resolveInvocation(tool string, args []string) (string, []string, error) {
	switch tool {
	case "fastgit":
		if path, err := exec.LookPath("fastgit"); err == nil {
			return path, args, nil
		}
		if path, err := exec.LookPath("go"); err == nil {
			fallback := []string{"run", "."}
			fallback = append(fallback, args...)
			return path, fallback, nil
		}
		return "", nil, errors.New("cannot find fastgit binary or go toolchain in PATH")
	case "git", "gh":
		path, err := exec.LookPath(tool)
		if err != nil {
			return "", nil, fmt.Errorf("cannot find %s in PATH", tool)
		}
		return path, args, nil
	default:
		return "", nil, fmt.Errorf("unsupported tool: %s", tool)
	}
}
