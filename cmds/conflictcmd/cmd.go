package conflictcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pubgo/fastgit/pkg/gitconflict"
	"github.com/pubgo/redant"
)

// New creates the conflict command group.
func New() *redant.Command {
	root := &redant.Command{
		Use:   "conflict",
		Short: "冲突检测、分组摘要与文件处理",
		Long:  "在 pull/rebase/merge 冲突时输出结构化摘要，并辅助打开冲突文件。",
	}

	root.Children = []*redant.Command{
		newSummaryCommand(),
		newListCommand(),
		newOpenCommand(),
	}

	root.Handler = newSummaryCommand().Handler
	return root
}

func newSummaryCommand() *redant.Command {
	var repo string
	return &redant.Command{
		Use:   "summary",
		Short: "输出冲突文件分组与处理建议（默认）",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}
			snap, err := gitconflict.BuildSnapshot(ctx, repoRoot)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, snap.Summary)
			if len(snap.Files) > 0 {
				return fmt.Errorf("%d conflicted file(s) remain", len(snap.Files))
			}
			return nil
		},
	}
}

func newListCommand() *redant.Command {
	var repo string
	return &redant.Command{
		Use:   "list",
		Short: "列出冲突文件",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}
			files, err := gitconflict.ListFiles(ctx, repoRoot)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "no conflicts")
				return nil
			}
			for _, file := range files {
				_, _ = fmt.Fprintln(inv.Stdout, file)
			}
			return nil
		},
	}
}

func newOpenCommand() *redant.Command {
	var repo string
	return &redant.Command{
		Use:   "open",
		Short: "在 $EDITOR 中打开全部冲突文件",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}
			files, err := gitconflict.ListFiles(ctx, repoRoot)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "no conflicts to open")
				return nil
			}

			editor := resolveEditor()
			for _, file := range files {
				fullPath := file
				if !strings.HasPrefix(file, "/") {
					fullPath = strings.TrimRight(repoRoot, "/") + "/" + file
				}
				_, _ = fmt.Fprintf(inv.Stdout, "opening %s\n", file)
				cmd := exec.CommandContext(ctx, editor, fullPath)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("open %s: %w", file, err)
				}
			}
			return nil
		},
	}
}

func resolveRepoRoot(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return repo, nil
}

func resolveEditor() string {
	if e := strings.TrimSpace(os.Getenv("EDITOR")); e != "" {
		return e
	}
	for _, candidate := range []string{"code", "vim", "nano", "vi"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "vi"
}
