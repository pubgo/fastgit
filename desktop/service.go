package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	transport "github.com/go-git/go-git/v6/plumbing/transport"
	httptransport "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	gitsshknownhosts "github.com/go-git/go-git/v6/plumbing/transport/ssh/knownhosts"
	"github.com/google/go-github/v71/github"
	"github.com/kevinburke/ssh_config"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
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
	Fields      []ActionField `json:"fields"`
}

type DesktopModule struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Actions     []ModuleAction `json:"actions"`
}

type GitHubAuthStatus struct {
	Configured bool   `json:"configured"`
	Source     string `json:"source"`
	Message    string `json:"message"`
}

type ActionRunRequest struct {
	ModuleID string            `json:"moduleID"`
	ActionID string            `json:"actionID"`
	Values   map[string]string `json:"values"`
}

type FastgitService struct {
	repoRoot    string
	githubToken string
}

type desktopSSHAuth struct {
	user            string
	methods         []gossh.AuthMethod
	hostAlias       string
	knownHostsFiles []string
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
	if _, err := git.PlainOpenWithOptions(abs, &git.PlainOpenOptions{DetectDotGit: true}); err != nil {
		return fmt.Errorf("%s is not a git repository: %w", abs, err)
	}
	s.repoRoot = abs
	return nil
}

func (s *FastgitService) SetGitHubToken(token string) {
	s.githubToken = strings.TrimSpace(token)
}

func (s *FastgitService) GetGitHubAuthStatus() GitHubAuthStatus {
	token, source := s.githubTokenValue()
	if token == "" {
		return GitHubAuthStatus{
			Configured: false,
			Source:     "none",
			Message:    "未配置 GitHub Token",
		}
	}

	repo, err := s.openRepo()
	if err != nil {
		return GitHubAuthStatus{
			Configured: true,
			Source:     source,
			Message:    fmt.Sprintf("Token 已就绪，但仓库不可用: %v", err),
		}
	}
	remote, err := repo.Remote("origin")
	if err != nil || remote == nil || len(remote.Config().URLs) == 0 {
		return GitHubAuthStatus{
			Configured: true,
			Source:     source,
			Message:    "Token 已就绪，但 origin remote 不可用",
		}
	}
	owner, repoName, err := parseGitHubRemote(remote.Config().URLs[0])
	if err != nil {
		return GitHubAuthStatus{
			Configured: true,
			Source:     source,
			Message:    fmt.Sprintf("Token 已就绪，但远端不是受支持的 GitHub 仓库: %v", err),
		}
	}

	label := "环境变量"
	if source == "session" {
		label = "当前会话"
	}
	return GitHubAuthStatus{
		Configured: true,
		Source:     source,
		Message:    fmt.Sprintf("GitHub 已连接: %s/%s (%s)", owner, repoName, label),
	}
}

func (s *FastgitService) GetModules() []DesktopModule {
	return []DesktopModule{
		{
			ID:          "repo",
			Title:       "仓库管理",
			Description: "状态 / 拉取 / 推送",
			Actions: []ModuleAction{
				{ID: "repo_status", Title: "状态", Description: "显示工作区状态"},
				{ID: "repo_pull", Title: "拉取", Description: "拉取当前分支"},
				{ID: "repo_push", Title: "推送", Description: "推送当前分支"},
			},
		},
		{
			ID:          "branch",
			Title:       "分支管理",
			Description: "列出 / 创建 / 切换 / 删除 / 强制对齐",
			Actions: []ModuleAction{
				{ID: "branch_list", Title: "列出本地分支", Description: "显示本地分支列表"},
				{ID: "branch_create", Title: "创建分支", Description: "创建新分支", Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "feature/my-branch", Required: true}}},
				{ID: "branch_checkout", Title: "切换分支", Description: "切换到指定分支", Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "main", Required: true}}},
				{ID: "branch_delete", Title: "删除分支", Description: "删除指定分支", Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "feature/my-branch", Required: true}}},
				{ID: "branch_force_sync", Title: "强制对齐远端", Description: "丢弃本地改动并强制对齐 origin/<branch>", Fields: []ActionField{{Key: "name", Label: "分支名", Placeholder: "main", Required: true}}},
			},
		},
		{
			ID:          "worktree",
			Title:       "Worktree 管理",
			Description: "列出 / 创建 / 删除",
			Actions: []ModuleAction{
				{ID: "worktree_list", Title: "列出 worktree", Description: "查看所有 worktree"},
				{ID: "worktree_create", Title: "创建 worktree", Description: "根据 issue/branch 创建", Fields: []ActionField{{Key: "target", Label: "issue/branch", Placeholder: "123 或 feature/abc", Required: true}, {Key: "base", Label: "base", Placeholder: "main", Required: true, Default: "main"}}},
				{ID: "worktree_remove", Title: "删除 worktree", Description: "删除指定 worktree", Fields: []ActionField{{Key: "target", Label: "issue/branch", Placeholder: "123 或 feature/abc", Required: true}}},
			},
		},
		{
			ID:          "issue",
			Title:       "Issue 管理",
			Description: "GitHub API（需 GITHUB_TOKEN）",
			Actions: []ModuleAction{
				{ID: "issue_list", Title: "列出 issue", Description: "列出 open issue"},
				{ID: "issue_view", Title: "查看 issue", Description: "查看 issue 详情", Fields: []ActionField{{Key: "id", Label: "Issue ID", Placeholder: "123", Required: true}}},
				{ID: "issue_create", Title: "创建 issue", Description: "创建 issue", Fields: []ActionField{{Key: "title", Label: "标题", Placeholder: "Issue title", Required: true}, {Key: "body", Label: "正文", Placeholder: "Issue body", Required: true}}},
			},
		},
		{
			ID:          "pr",
			Title:       "PR 管理",
			Description: "GitHub API（需 GITHUB_TOKEN）",
			Actions: []ModuleAction{
				{ID: "pr_status", Title: "PR 状态", Description: "查看当前分支对应 PR"},
				{ID: "pr_create", Title: "创建 PR", Description: "为当前分支创建 PR", Fields: []ActionField{
					{Key: "title", Label: "标题", Placeholder: "Update feature/my-branch"},
					{Key: "body", Label: "正文", Placeholder: "PR body"},
					{Key: "base", Label: "目标分支", Placeholder: "main"},
				}},
				{ID: "pr_sync", Title: "同步 PR 内容", Description: "更新当前分支 PR 标题与正文", Fields: []ActionField{
					{Key: "title", Label: "标题", Placeholder: "Update feature/my-branch"},
					{Key: "body", Label: "正文", Placeholder: "PR body"},
				}},
				{ID: "pr_merge", Title: "合并 PR", Description: "合并当前分支 PR", Fields: []ActionField{{Key: "method", Label: "merge method", Placeholder: "squash|merge|rebase", Required: true, Default: "squash"}}},
			},
		},
		{
			ID:          "tag",
			Title:       "Tag 管理",
			Description: "列出 / 创建 / 推送",
			Actions: []ModuleAction{
				{ID: "tag_list", Title: "列出 tag", Description: "列出本地 tags"},
				{ID: "tag_publish", Title: "创建 tag", Description: "在当前 HEAD 创建 tag", Fields: []ActionField{{Key: "name", Label: "Tag", Placeholder: "v1.2.3", Required: true}}},
				{ID: "tag_push", Title: "推送 tag", Description: "推送指定 tag", Fields: []ActionField{{Key: "name", Label: "Tag", Placeholder: "v1.2.3", Required: true}}},
			},
		},
	}
}

func (s *FastgitService) RunAction(req ActionRunRequest) (CommandResult, error) {
	_, action, err := s.findAction(req.ModuleID, req.ActionID)
	if err != nil {
		return CommandResult{}, err
	}
	values := normalizeValues(req.Values)

	started := time.Now().UTC()
	result := CommandResult{
		Command:   fmt.Sprintf("sdk/%s/%s", req.ModuleID, req.ActionID),
		StartedAt: started.Format(time.RFC3339),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	output, err := s.dispatchAction(ctx, action.ID, values)
	result.EndedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		result.ExitCode = 1
		if strings.TrimSpace(output) == "" {
			output = err.Error()
		}
		result.Output = output
		return result, nil
	}

	result.ExitCode = 0
	result.Output = strings.TrimSpace(output)
	if result.Output == "" {
		result.Output = "ok"
	}
	return result, nil
}

func normalizeValues(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = strings.TrimSpace(v)
	}
	return out
}

func (s *FastgitService) dispatchAction(ctx context.Context, actionID string, values map[string]string) (string, error) {
	switch actionID {
	case "repo_status":
		return s.repoStatus(ctx)
	case "repo_pull":
		return s.repoPull(ctx)
	case "repo_push":
		return s.repoPush(ctx)
	case "branch_list":
		return s.branchList(ctx)
	case "branch_create":
		name, err := requiredValue(values, "name", "分支名")
		if err != nil {
			return "", err
		}
		return s.branchCreate(ctx, name)
	case "branch_checkout":
		name, err := requiredValue(values, "name", "分支名")
		if err != nil {
			return "", err
		}
		return s.branchCheckout(ctx, name)
	case "branch_delete":
		name, err := requiredValue(values, "name", "分支名")
		if err != nil {
			return "", err
		}
		return s.branchDelete(ctx, name)
	case "branch_force_sync":
		name, err := requiredValue(values, "name", "分支名")
		if err != nil {
			return "", err
		}
		return s.branchForceSync(ctx, name)
	case "worktree_list":
		return s.worktreeList(ctx)
	case "worktree_create":
		target, err := requiredValue(values, "target", "issue/branch")
		if err != nil {
			return "", err
		}
		base := optionalValue(values, "base", "main")
		return s.worktreeCreate(ctx, target, base)
	case "worktree_remove":
		target, err := requiredValue(values, "target", "issue/branch")
		if err != nil {
			return "", err
		}
		return s.worktreeRemove(ctx, target)
	case "issue_list":
		return s.issueList(ctx)
	case "issue_view":
		id, err := requiredInt(values, "id", "Issue ID")
		if err != nil {
			return "", err
		}
		return s.issueView(ctx, id)
	case "issue_create":
		title, err := requiredValue(values, "title", "标题")
		if err != nil {
			return "", err
		}
		body, err := requiredValue(values, "body", "正文")
		if err != nil {
			return "", err
		}
		return s.issueCreate(ctx, title, body)
	case "pr_status":
		return s.prStatus(ctx)
	case "pr_create":
		title := optionalValue(values, "title", "")
		body := optionalValue(values, "body", "")
		base := optionalValue(values, "base", "")
		return s.prCreate(ctx, title, body, base)
	case "pr_sync":
		title := optionalValue(values, "title", "")
		body := optionalValue(values, "body", "")
		return s.prSync(ctx, title, body)
	case "pr_merge":
		method := optionalValue(values, "method", "squash")
		return s.prMerge(ctx, method)
	case "tag_list":
		return s.tagList(ctx)
	case "tag_publish":
		name, err := requiredValue(values, "name", "Tag")
		if err != nil {
			return "", err
		}
		return s.tagPublish(ctx, name)
	case "tag_push":
		name, err := requiredValue(values, "name", "Tag")
		if err != nil {
			return "", err
		}
		return s.tagPush(ctx, name)
	default:
		return "", fmt.Errorf("unsupported action: %s", actionID)
	}
}

func requiredValue(values map[string]string, key, label string) (string, error) {
	v := strings.TrimSpace(values[key])
	if v == "" {
		return "", fmt.Errorf("%s 不能为空", label)
	}
	return v, nil
}

func optionalValue(values map[string]string, key, def string) string {
	v := strings.TrimSpace(values[key])
	if v == "" {
		return def
	}
	return v
}

func requiredInt(values map[string]string, key, label string) (int, error) {
	raw, err := requiredValue(values, key, label)
	if err != nil {
		return 0, err
	}
	n, convErr := strconvAtoi(raw)
	if convErr != nil {
		return 0, fmt.Errorf("%s 不是有效数字", label)
	}
	return n, nil
}

func strconvAtoi(v string) (int, error) {
	var out int
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid number")
		}
		out = out*10 + int(ch-'0')
	}
	return out, nil
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

func (s *FastgitService) openRepo() (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(s.repoRoot, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open repo failed: %w", err)
	}
	return repo, nil
}

func (s *FastgitService) currentBranch(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("detached HEAD")
	}
	return head.Name().Short(), nil
}

func (s *FastgitService) repoStatus(ctx context.Context) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	status, err := wt.Status()
	if err != nil {
		return "", err
	}
	if status.IsClean() {
		return "working tree clean", nil
	}

	keys := make([]string, 0, len(status))
	for path := range status {
		keys = append(keys, path)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, path := range keys {
		st := status[path]
		fmt.Fprintf(&b, "%c%c %s\n", st.Staging, st.Worktree, path)
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *FastgitService) repoPull(ctx context.Context) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	branch, err := s.currentBranch(repo)
	if err != nil {
		return "", err
	}
	if isSSHRemoteRepo(repo) {
		out, err := s.gitInRepo(ctx, "pull", "origin", branch)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return "pull completed", nil
		}
		return strings.TrimSpace(out), nil
	}
	opts := &git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	}
	opts.ClientOptions, err = s.clientOptions(repo)
	if err != nil {
		return "", err
	}
	if err := wt.PullContext(ctx, opts); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "already up-to-date", nil
		}
		return "", err
	}
	return "pull completed", nil
}

func (s *FastgitService) repoPush(ctx context.Context) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	if isSSHRemoteRepo(repo) {
		out, err := s.gitInRepo(ctx, "push", "origin", "HEAD")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return "push completed", nil
		}
		return strings.TrimSpace(out), nil
	}
	clientOptions, err := s.clientOptions(repo)
	if err != nil {
		return "", err
	}
	err = repo.PushContext(ctx, &git.PushOptions{RemoteName: "origin", ClientOptions: clientOptions})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "already up-to-date", nil
		}
		return "", err
	}
	return "push completed", nil
}

func (s *FastgitService) branchList(ctx context.Context) (string, error) {
	_ = ctx
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	current, err := s.currentBranch(repo)
	if err != nil {
		return "", err
	}
	iter, err := repo.Branches()
	if err != nil {
		return "", err
	}
	branches := make([]string, 0)
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	})
	sort.Strings(branches)
	var b strings.Builder
	for _, name := range branches {
		prefix := "  "
		if name == current {
			prefix = "* "
		}
		b.WriteString(prefix + name + "\n")
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *FastgitService) branchCreate(ctx context.Context, name string) (string, error) {
	_ = ctx
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("detached HEAD")
	}
	refName := plumbing.NewBranchReferenceName(name)
	if _, err := repo.Reference(refName, true); err == nil {
		return "", fmt.Errorf("branch exists: %s", name)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, head.Hash())); err != nil {
		return "", err
	}
	return fmt.Sprintf("branch created: %s", name), nil
}

func (s *FastgitService) branchCheckout(ctx context.Context, name string) (string, error) {
	_ = ctx
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	refName := plumbing.NewBranchReferenceName(name)
	if err := wt.Checkout(&git.CheckoutOptions{Branch: refName}); err != nil {
		return "", err
	}
	return fmt.Sprintf("checked out: %s", name), nil
}

func (s *FastgitService) branchDelete(ctx context.Context, name string) (string, error) {
	_ = ctx
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	current, err := s.currentBranch(repo)
	if err != nil {
		return "", err
	}
	if current == name {
		return "", fmt.Errorf("cannot delete current branch: %s", name)
	}
	if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(name)); err != nil {
		return "", err
	}
	return fmt.Sprintf("branch deleted: %s", name), nil
}

func (s *FastgitService) branchForceSync(ctx context.Context, name string) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	current, err := s.currentBranch(repo)
	if err != nil {
		return "", err
	}
	if _, err := repo.Reference(plumbing.NewBranchReferenceName(name), true); err != nil {
		return "", fmt.Errorf("local branch not found: %s", name)
	}

	remoteRef := "origin/" + name
	if _, err := s.gitInRepo(ctx, "fetch", "--prune", "origin"); err != nil {
		return "", err
	}
	if _, err := s.gitInRepo(ctx, "rev-parse", "--verify", "refs/remotes/"+remoteRef); err != nil {
		return "", fmt.Errorf("remote branch not found: %s", remoteRef)
	}

	if current != name {
		if _, err := s.gitInRepo(ctx, "clean", "-fd"); err != nil {
			return "", err
		}
		if _, err := s.gitInRepo(ctx, "checkout", "-f", name); err != nil {
			return "", err
		}
	}
	if _, err := s.gitInRepo(ctx, "reset", "--hard", remoteRef); err != nil {
		return "", err
	}
	if _, err := s.gitInRepo(ctx, "clean", "-fd"); err != nil {
		return "", err
	}

	head, err := s.gitInRepo(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"force aligned: %s -> %s\nHEAD=%s\nlocal changes discarded\nuntracked files removed",
		name,
		remoteRef,
		strings.TrimSpace(head),
	), nil
}

func (s *FastgitService) worktreeList(ctx context.Context) (string, error) {
	out, err := s.gitInRepo(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "no worktree", nil
	}
	return strings.TrimSpace(out), nil
}

func (s *FastgitService) worktreeCreate(ctx context.Context, target, base string) (string, error) {
	repoTop, err := s.gitInRepo(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	repoTop = strings.TrimSpace(repoTop)
	repoName := filepath.Base(repoTop)
	branchName, suffix := determineWorktreeNames(target)
	worktreePath := filepath.Join(filepath.Dir(repoTop), fmt.Sprintf("%s-%s", repoName, suffix))
	if _, err := s.gitInRepo(ctx, "worktree", "add", worktreePath, "-b", branchName, base); err != nil {
		return "", err
	}
	return fmt.Sprintf("created worktree: %s", worktreePath), nil
}

func (s *FastgitService) worktreeRemove(ctx context.Context, target string) (string, error) {
	repoTop, err := s.gitInRepo(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	repoTop = strings.TrimSpace(repoTop)
	repoName := filepath.Base(repoTop)
	_, suffix := determineWorktreeNames(target)
	worktreePath := filepath.Join(filepath.Dir(repoTop), fmt.Sprintf("%s-%s", repoName, suffix))
	if _, err := s.gitInRepo(ctx, "worktree", "remove", worktreePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("removed worktree: %s", worktreePath), nil
}

func determineWorktreeNames(input string) (branchName, dirSuffix string) {
	if strings.Contains(input, "/") {
		branchName = input
		dirSuffix = sanitizeBranchName(input)
		return
	}
	branchName = input + "/impl"
	dirSuffix = input
	return
}

func sanitizeBranchName(v string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		"*", "-",
		"?", "-",
		":", "-",
		"<", "-",
		">", "-",
		"\"", "-",
		"|", "-",
	)
	s := replacer.Replace(v)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func (s *FastgitService) gitInRepo(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (s *FastgitService) issueList(ctx context.Context) (string, error) {
	owner, repoName, client, err := s.githubClient(ctx)
	if err != nil {
		return "", err
	}
	issues, _, err := client.Issues.ListByRepo(ctx, owner, repoName, &github.IssueListByRepoOptions{State: "open", ListOptions: github.ListOptions{PerPage: 30}})
	if err != nil {
		return "", err
	}
	if len(issues) == 0 {
		return "no open issues", nil
	}
	var b strings.Builder
	for _, it := range issues {
		if it.GetPullRequestLinks() != nil {
			continue
		}
		fmt.Fprintf(&b, "#%d [%s] %s\n", it.GetNumber(), it.GetState(), it.GetTitle())
	}
	if strings.TrimSpace(b.String()) == "" {
		return "no open issues", nil
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *FastgitService) issueView(ctx context.Context, number int) (string, error) {
	owner, repoName, client, err := s.githubClient(ctx)
	if err != nil {
		return "", err
	}
	issue, _, err := client.Issues.Get(ctx, owner, repoName, number)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"#%d [%s]\n%s\n%s\n\n%s",
		issue.GetNumber(),
		issue.GetState(),
		issue.GetTitle(),
		issue.GetHTMLURL(),
		strings.TrimSpace(issue.GetBody()),
	), nil
}

func (s *FastgitService) issueCreate(ctx context.Context, title, body string) (string, error) {
	owner, repoName, client, err := s.githubClient(ctx)
	if err != nil {
		return "", err
	}
	req := &github.IssueRequest{Title: github.Ptr(title), Body: github.Ptr(body)}
	issue, _, err := client.Issues.Create(ctx, owner, repoName, req)
	if err != nil {
		return "", err
	}
	return issue.GetHTMLURL(), nil
}

func (s *FastgitService) prStatus(ctx context.Context) (string, error) {
	owner, repoName, client, branch, err := s.githubBranchClient(ctx)
	if err != nil {
		return "", err
	}
	pr, err := s.findPRForBranch(ctx, owner, repoName, client, branch)
	if err != nil {
		return "", err
	}
	if pr == nil {
		return "no open PR for current branch", nil
	}
	body := strings.TrimSpace(pr.GetBody())
	return fmt.Sprintf(
		"#%d [%s]\n%s\n%s\nbase: %s\nhead: %s\ndraft: %t\n\n%s",
		pr.GetNumber(),
		pr.GetState(),
		pr.GetTitle(),
		pr.GetHTMLURL(),
		pr.GetBase().GetRef(),
		pr.GetHead().GetRef(),
		pr.GetDraft(),
		body,
	), nil
}

func (s *FastgitService) prCreate(ctx context.Context, title, body, base string) (string, error) {
	owner, repoName, client, branch, err := s.githubBranchClient(ctx)
	if err != nil {
		return "", err
	}
	if existing, err := s.findPRForBranch(ctx, owner, repoName, client, branch); err == nil && existing != nil {
		return fmt.Sprintf("PR already exists: %s", existing.GetHTMLURL()), nil
	}

	repoInfo, _, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return "", err
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = repoInfo.GetDefaultBranch()
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Update " + branch
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = "Generated by fastgit desktop SDK layer"
	}

	newPR := &github.NewPullRequest{Title: github.Ptr(title), Head: github.Ptr(branch), Base: github.Ptr(base), Body: github.Ptr(body)}
	created, _, err := client.PullRequests.Create(ctx, owner, repoName, newPR)
	if err != nil {
		return "", err
	}
	return created.GetHTMLURL(), nil
}

func (s *FastgitService) prSync(ctx context.Context, title, body string) (string, error) {
	owner, repoName, client, branch, err := s.githubBranchClient(ctx)
	if err != nil {
		return "", err
	}
	pr, err := s.findPRForBranch(ctx, owner, repoName, client, branch)
	if err != nil {
		return "", err
	}
	if pr == nil {
		return "no open PR for current branch", nil
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Update " + branch
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = "Updated by fastgit desktop SDK layer"
	}
	edited := &github.PullRequest{Title: github.Ptr(title), Body: github.Ptr(body)}
	updated, _, err := client.PullRequests.Edit(ctx, owner, repoName, pr.GetNumber(), edited)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PR updated: %s", updated.GetHTMLURL()), nil
}

func (s *FastgitService) prMerge(ctx context.Context, method string) (string, error) {
	owner, repoName, client, branch, err := s.githubBranchClient(ctx)
	if err != nil {
		return "", err
	}
	pr, err := s.findPRForBranch(ctx, owner, repoName, client, branch)
	if err != nil {
		return "", err
	}
	if pr == nil {
		return "", fmt.Errorf("no open PR for current branch")
	}
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		method = "squash"
	}
	if method != "squash" && method != "merge" && method != "rebase" {
		return "", fmt.Errorf("unsupported merge method: %s", method)
	}
	res, _, err := client.PullRequests.Merge(ctx, owner, repoName, pr.GetNumber(), "", &github.PullRequestOptions{MergeMethod: method})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("merged=%v message=%s", res.GetMerged(), res.GetMessage()), nil
}

func (s *FastgitService) tagList(ctx context.Context) (string, error) {
	_ = ctx
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	iter, err := repo.Tags()
	if err != nil {
		return "", err
	}
	tags := make([]string, 0)
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		tags = append(tags, ref.Name().Short())
		return nil
	})
	if len(tags) == 0 {
		return "no tags", nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	return strings.Join(tags, "\n"), nil
}

func (s *FastgitService) tagPublish(ctx context.Context, name string) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	if _, err := repo.CreateTag(name, head.Hash(), nil); err != nil {
		return "", err
	}
	return fmt.Sprintf("tag created: %s", name), nil
}

func (s *FastgitService) tagPush(ctx context.Context, name string) (string, error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", err
	}
	if isSSHRemoteRepo(repo) {
		out, err := s.gitInRepo(ctx, "push", "origin", fmt.Sprintf("refs/tags/%[1]s:refs/tags/%[1]s", name))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Sprintf("tag pushed: %s", name), nil
		}
		return strings.TrimSpace(out), nil
	}
	refSpec := gitconfig.RefSpec(fmt.Sprintf("refs/tags/%[1]s:refs/tags/%[1]s", name))
	clientOptions, err := s.clientOptions(repo)
	if err != nil {
		return "", err
	}
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName:    "origin",
		RefSpecs:      []gitconfig.RefSpec{refSpec},
		ClientOptions: clientOptions,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "already up-to-date", nil
		}
		return "", err
	}
	return fmt.Sprintf("tag pushed: %s", name), nil
}

func (s *FastgitService) githubClient(ctx context.Context) (owner, repoName string, client *github.Client, err error) {
	repo, err := s.openRepo()
	if err != nil {
		return "", "", nil, err
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", nil, fmt.Errorf("origin remote not found: %w", err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", "", nil, fmt.Errorf("origin remote has no URL")
	}
	owner, repoName, err = parseGitHubRemote(urls[0])
	if err != nil {
		return "", "", nil, err
	}
	token, _ := s.githubTokenValue()
	if token == "" {
		return "", "", nil, fmt.Errorf("GITHUB_TOKEN/GH_TOKEN is required for GitHub API operations")
	}
	tok := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, tok)
	return owner, repoName, github.NewClient(httpClient), nil
}

func (s *FastgitService) githubBranchClient(ctx context.Context) (owner, repoName string, client *github.Client, branch string, err error) {
	owner, repoName, client, err = s.githubClient(ctx)
	if err != nil {
		return "", "", nil, "", err
	}
	repo, err := s.openRepo()
	if err != nil {
		return "", "", nil, "", err
	}
	branch, err = s.currentBranch(repo)
	if err != nil {
		return "", "", nil, "", err
	}
	return owner, repoName, client, branch, nil
}

func (s *FastgitService) findPRForBranch(ctx context.Context, owner, repoName string, client *github.Client, branch string) (*github.PullRequest, error) {
	prs, _, err := client.PullRequests.List(ctx, owner, repoName, &github.PullRequestListOptions{State: "open", Head: owner + ":" + branch, ListOptions: github.ListOptions{PerPage: 20}})
	if err != nil {
		return nil, err
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return prs[0], nil
}

func parseGitHubRemote(remote string) (owner, repo string, err error) {
	remote = strings.TrimSpace(strings.TrimSuffix(remote, ".git"))
	var path string

	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		path = strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		path = strings.TrimPrefix(remote, "ssh://git@github.com/")
	default:
		u, parseErr := neturl.Parse(remote)
		if parseErr != nil {
			return "", "", fmt.Errorf("parse remote URL failed: %w", parseErr)
		}
		if !strings.EqualFold(u.Hostname(), "github.com") {
			return "", "", fmt.Errorf("only github.com is supported in desktop GitHub API mode")
		}
		path = strings.TrimPrefix(u.Path, "/")
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub remote: %s", remote)
	}
	return parts[0], parts[1], nil
}

func isSSHRemoteRepo(repo *git.Repository) bool {
	if repo == nil {
		return false
	}
	remote, err := repo.Remote("origin")
	if err != nil || remote == nil || len(remote.Config().URLs) == 0 {
		return false
	}
	parsedURL, err := transport.ParseURL(strings.TrimSpace(remote.Config().URLs[0]))
	if err != nil {
		return false
	}
	return strings.EqualFold(parsedURL.Scheme, "ssh")
}

func (s *FastgitService) clientOptions(repo *git.Repository) ([]client.Option, error) {
	remote, err := repo.Remote("origin")
	if err != nil || remote == nil || len(remote.Config().URLs) == 0 {
		return nil, nil
	}
	rawURL := strings.TrimSpace(remote.Config().URLs[0])
	parsedURL, err := transport.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse remote URL failed: %w", err)
	}

	switch strings.ToLower(parsedURL.Scheme) {
	case "http", "https":
		token, _ := s.githubTokenValue()
		if token == "" {
			return nil, nil
		}
		return []client.Option{client.WithHTTPAuth(&httptransport.TokenAuth{Token: token})}, nil
	case "ssh":
		auth, err := buildSSHAuth(parsedURL)
		if err != nil {
			return nil, err
		}
		return []client.Option{client.WithSSHAuth(auth)}, nil
	default:
		return nil, nil
	}
}

func buildSSHAuth(remoteURL *neturl.URL) (client.SSHAuth, error) {
	if remoteURL == nil {
		return nil, errors.New("ssh remote URL is nil")
	}

	hostAlias := remoteURL.Hostname()
	user := "git"
	if remoteURL.User != nil && remoteURL.User.Username() != "" {
		user = remoteURL.User.Username()
	} else if configuredUser := strings.TrimSpace(ssh_config.Get(hostAlias, "User")); configuredUser != "" {
		user = configuredUser
	}

	identitiesOnly := strings.EqualFold(strings.TrimSpace(ssh_config.Get(hostAlias, "IdentitiesOnly")), "yes")
	identityFiles := normalizeSSHConfigPaths(hostAlias, "IdentityFile")
	knownHostsFiles := append(
		normalizeSSHConfigPaths(hostAlias, "UserKnownHostsFile"),
		normalizeSSHConfigPaths(hostAlias, "GlobalKnownHostsFile")...,
	)
	knownHostsFiles = uniqueStrings(knownHostsFiles)

	methods, keyErrs := loadSSHKeyAuthMethods(user, identityFiles)

	var agentErr error
	if !identitiesOnly {
		agentAuth, err := gitssh.NewSSHAgentAuth(user)
		if err == nil {
			methods = append(methods, gossh.PublicKeysCallback(agentAuth.Callback))
		} else {
			agentErr = err
		}
	}

	if len(methods) == 0 {
		var reasons []string
		if len(keyErrs) > 0 {
			reasons = append(reasons, "key files: "+strings.Join(keyErrs, "; "))
		}
		if agentErr != nil {
			reasons = append(reasons, "ssh-agent: "+agentErr.Error())
		}
		if len(reasons) == 0 {
			reasons = append(reasons, "no usable SSH key or ssh-agent identity found")
		}
		return nil, fmt.Errorf("ssh auth unavailable for %s: %s", hostAlias, strings.Join(reasons, " | "))
	}

	return &desktopSSHAuth{
		user:            user,
		methods:         methods,
		hostAlias:       hostAlias,
		knownHostsFiles: knownHostsFiles,
	}, nil
}

func (a *desktopSSHAuth) ClientConfig(_ context.Context, req *transport.Request) (*gossh.ClientConfig, error) {
	cfg := &gossh.ClientConfig{
		User: a.user,
		Auth: a.methods,
	}
	if len(a.knownHostsFiles) == 0 {
		return cfg, nil
	}

	usableKnownHostsFiles, err := existingFiles(a.knownHostsFiles)
	if err != nil {
		return nil, fmt.Errorf("inspect known_hosts failed: %w", err)
	}
	if len(usableKnownHostsFiles) == 0 {
		return cfg, nil
	}

	knownHostsDB, err := gitsshknownhosts.NewDB(usableKnownHostsFiles...)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts failed: %w", err)
	}
	hostWithPort := a.resolveHostWithPort(req)
	cfg.HostKeyCallback = knownHostsDB.HostKeyCallback()
	cfg.HostKeyAlgorithms = knownHostsDB.HostKeyAlgorithms(hostWithPort)
	return cfg, nil
}

func (a *desktopSSHAuth) resolveHostWithPort(req *transport.Request) string {
	hostAlias := a.hostAlias
	if hostAlias == "" && req != nil && req.URL != nil {
		hostAlias = req.URL.Hostname()
	}

	host := strings.TrimSpace(ssh_config.Get(hostAlias, "Hostname"))
	if host == "" && req != nil && req.URL != nil {
		host = req.URL.Hostname()
	}
	if host == "" {
		host = hostAlias
	}

	port := ""
	if req != nil && req.URL != nil {
		port = req.URL.Port()
	}
	if port == "" {
		port = strings.TrimSpace(ssh_config.Get(hostAlias, "Port"))
	}
	if port == "" {
		port = "22"
	}

	return net.JoinHostPort(host, port)
}

func loadSSHKeyAuthMethods(user string, identityFiles []string) ([]gossh.AuthMethod, []string) {
	methods := make([]gossh.AuthMethod, 0, len(identityFiles))
	errs := make([]string, 0)
	for _, identityFile := range uniqueStrings(identityFiles) {
		info, err := os.Stat(identityFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errs = append(errs, fmt.Sprintf("%s: %v", identityFile, err))
			continue
		}
		if info.IsDir() {
			continue
		}
		auth, err := gitssh.NewPublicKeysFromFile(user, identityFile, "")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", identityFile, err))
			continue
		}
		signer, err := preferredSigner(auth.Signer)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", identityFile, err))
			continue
		}
		methods = append(methods, gossh.PublicKeys(signer))
	}
	return methods, errs
}

func preferredSigner(signer gossh.Signer) (gossh.Signer, error) {
	if signer == nil {
		return nil, errors.New("ssh signer is nil")
	}
	if signer.PublicKey().Type() != gossh.KeyAlgoRSA {
		return signer, nil
	}

	algorithmSigner, ok := signer.(gossh.AlgorithmSigner)
	if !ok {
		return signer, nil
	}

	return gossh.NewSignerWithAlgorithms(algorithmSigner, []string{
		gossh.KeyAlgoRSASHA512,
		gossh.KeyAlgoRSASHA256,
	})
}

func normalizeSSHConfigPaths(hostAlias, key string) []string {
	values := ssh_config.GetAll(hostAlias, key)
	if len(values) == 0 {
		return nil
	}
	home, _ := os.UserHomeDir()
	return normalizeSSHPaths(values, home)
}

func normalizeSSHPaths(values []string, home string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range splitSSHPathList(value) {
			path := strings.TrimSpace(part)
			if home != "" && strings.HasPrefix(path, "~/") {
				path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
			}
			if home != "" && path == "~" {
				path = home
			}
			normalized = append(normalized, path)
		}
	}
	return normalized
}

func splitSSHPathList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parts := make([]string, 0, 1)
	var current strings.Builder
	var quote rune

	flush := func() {
		if current.Len() == 0 {
			return
		}
		parts = append(parts, current.String())
		current.Reset()
	}

	for _, r := range value {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return parts
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func existingFiles(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	files := make([]string, 0, len(paths))
	for _, path := range uniqueStrings(paths) {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				files = append(files, path)
			}
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return nil, err
	}

	return files, nil
}

func (s *FastgitService) githubTokenValue() (token string, source string) {
	if value := strings.TrimSpace(s.githubToken); value != "" {
		return value, "session"
	}
	if value := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); value != "" {
		return value, "env"
	}
	if value := strings.TrimSpace(os.Getenv("GH_TOKEN")); value != "" {
		return value, "env"
	}
	return "", "none"
}
