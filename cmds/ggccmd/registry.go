package ggccmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/pubgo/fastgit/utils"
)

type CommandHandler func(ctx context.Context, rest []string) error

type CommandEntry struct {
	Key         string
	Usage       string
	Description string
	Handler     CommandHandler
}

type Registry struct {
	entries []CommandEntry
	byKey   map[string]CommandEntry
}

func NewRegistry() *Registry {
	r := &Registry{byKey: make(map[string]CommandEntry)}

	r.Register(CommandEntry{
		Key:         "status",
		Usage:       "status",
		Description: "Show working tree status",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "status")
		},
	})

	r.Register(CommandEntry{
		Key:         "status short",
		Usage:       "status short",
		Description: "Show concise status",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "status", "--short")
		},
	})

	r.Register(CommandEntry{
		Key:         "add",
		Usage:       "add <file|.>",
		Description: "Stage files",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) == 0 {
				return fmt.Errorf("usage: add <file|.>")
			}

			return runGitCommand(ctx, append([]string{"add"}, rest...)...)
		},
	})

	r.Register(CommandEntry{
		Key:         "commit",
		Usage:       "commit <message>",
		Description: "Create a commit with message",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) == 0 {
				return fmt.Errorf("usage: commit <message>")
			}

			return runGitCommand(ctx, "commit", "-m", strings.Join(rest, " "))
		},
	})

	r.Register(CommandEntry{
		Key:         "log simple",
		Usage:       "log simple",
		Description: "Show simple commit log",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "log", "--oneline", "-20")
		},
	})

	r.Register(CommandEntry{
		Key:         "log graph",
		Usage:       "log graph",
		Description: "Show graph commit log",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "log", "--graph", "--decorate", "--oneline", "-30")
		},
	})

	r.Register(CommandEntry{
		Key:         "diff",
		Usage:       "diff",
		Description: "Show diff against HEAD",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "diff", "HEAD")
		},
	})

	r.Register(CommandEntry{
		Key:         "diff staged",
		Usage:       "diff staged",
		Description: "Show staged diff",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "diff", "--cached")
		},
	})

	r.Register(CommandEntry{
		Key:         "diff unstaged",
		Usage:       "diff unstaged",
		Description: "Show unstaged diff",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "diff")
		},
	})

	r.Register(CommandEntry{
		Key:         "branch current",
		Usage:       "branch current",
		Description: "Show current branch",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "branch", "--show-current")
		},
	})

	r.Register(CommandEntry{
		Key:         "branch list local",
		Usage:       "branch list local",
		Description: "List local branches",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "branch")
		},
	})

	r.Register(CommandEntry{
		Key:         "branch list remote",
		Usage:       "branch list remote",
		Description: "List remote branches",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "branch", "-r")
		},
	})

	r.Register(CommandEntry{
		Key:         "branch checkout",
		Usage:       "branch checkout <name>",
		Description: "Checkout branch",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: branch checkout <name>")
			}

			return runGitCommand(ctx, "checkout", rest[0])
		},
	})

	r.Register(CommandEntry{
		Key:         "branch checkout remote",
		Usage:       "branch checkout remote <name>",
		Description: "Checkout remote branch to local",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: branch checkout remote <name>")
			}

			remote := rest[0]
			if !strings.HasPrefix(remote, "origin/") {
				remote = "origin/" + remote
			}

			local := strings.TrimPrefix(remote, "origin/")
			return runGitCommand(ctx, "checkout", "-b", local, "--track", remote)
		},
	})

	r.Register(CommandEntry{
		Key:         "branch create",
		Usage:       "branch create <name>",
		Description: "Create and checkout new branch",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: branch create <name>")
			}

			return runGitCommand(ctx, "checkout", "-b", rest[0])
		},
	})

	r.Register(CommandEntry{
		Key:         "branch delete",
		Usage:       "branch delete <name>",
		Description: "Delete local branch",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: branch delete <name>")
			}

			return runGitCommand(ctx, "branch", "-d", rest[0])
		},
	})

	r.Register(CommandEntry{
		Key:         "fetch",
		Usage:       "fetch",
		Description: "Fetch from remote",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "fetch")
		},
	})

	r.Register(CommandEntry{
		Key:         "fetch prune",
		Usage:       "fetch prune",
		Description: "Fetch and prune stale refs",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "fetch", "--prune")
		},
	})

	r.Register(CommandEntry{
		Key:         "pull current",
		Usage:       "pull current",
		Description: "Pull current branch from origin",
		Handler: func(ctx context.Context, _ []string) error {
			branch, err := utils.GetCurrentBranchV1()
			if err != nil {
				return err
			}

			return runGitCommand(ctx, "pull", "origin", branch)
		},
	})

	r.Register(CommandEntry{
		Key:         "pull rebase",
		Usage:       "pull rebase",
		Description: "Pull with rebase",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "pull", "--rebase")
		},
	})

	r.Register(CommandEntry{
		Key:         "push current",
		Usage:       "push current",
		Description: "Push current branch to origin",
		Handler: func(ctx context.Context, _ []string) error {
			branch, err := utils.GetCurrentBranchV1()
			if err != nil {
				return err
			}

			return runGitCommand(ctx, "push", "origin", branch)
		},
	})

	r.Register(CommandEntry{
		Key:         "push force",
		Usage:       "push force",
		Description: "Force push current branch",
		Handler: func(ctx context.Context, _ []string) error {
			branch, err := utils.GetCurrentBranchV1()
			if err != nil {
				return err
			}

			return runGitCommand(ctx, "push", "--force-with-lease", "origin", branch)
		},
	})

	r.Register(CommandEntry{
		Key:         "rebase",
		Usage:       "rebase <upstream>",
		Description: "Rebase current branch",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: rebase <upstream>")
			}

			return runGitCommand(ctx, "rebase", rest[0])
		},
	})

	r.Register(CommandEntry{
		Key:         "rebase continue",
		Usage:       "rebase continue",
		Description: "Continue in-progress rebase",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "rebase", "--continue")
		},
	})

	r.Register(CommandEntry{
		Key:         "rebase abort",
		Usage:       "rebase abort",
		Description: "Abort in-progress rebase",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "rebase", "--abort")
		},
	})

	r.Register(CommandEntry{
		Key:         "rebase skip",
		Usage:       "rebase skip",
		Description: "Skip current patch in rebase",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "rebase", "--skip")
		},
	})

	r.Register(CommandEntry{
		Key:         "tag list",
		Usage:       "tag list",
		Description: "List tags",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "tag", "--sort=-committerdate")
		},
	})

	r.Register(CommandEntry{
		Key:         "tag show",
		Usage:       "tag show <tag>",
		Description: "Show tag info",
		Handler: func(ctx context.Context, rest []string) error {
			if len(rest) != 1 {
				return fmt.Errorf("usage: tag show <tag>")
			}

			return runGitCommand(ctx, "show", rest[0])
		},
	})

	r.Register(CommandEntry{
		Key:         "remote list",
		Usage:       "remote list",
		Description: "List remotes",
		Handler: func(ctx context.Context, _ []string) error {
			return runGitCommand(ctx, "remote", "-v")
		},
	})

	return r
}

func (r *Registry) Register(entry CommandEntry) {
	r.byKey[entry.Key] = entry
	r.entries = append(r.entries, entry)
}

func (r *Registry) Execute(ctx context.Context, args []string) error {
	max := len(args)
	if max > 4 {
		max = 4
	}

	for i := max; i > 0; i-- {
		key := strings.Join(args[:i], " ")
		if entry, ok := r.byKey[key]; ok {
			return entry.Handler(ctx, args[i:])
		}
	}

	return fmt.Errorf("unknown command: %s", strings.Join(args, " "))
}

func (r *Registry) List() []CommandEntry {
	out := make([]CommandEntry, len(r.entries))
	copy(out, r.entries)

	sort.Slice(out, func(i, j int) bool {
		return out[i].Usage < out[j].Usage
	})

	return out
}

func (r *Registry) Get(key string) (CommandEntry, bool) {
	entry, ok := r.byKey[key]
	return entry, ok
}

func runGitCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return nil
}
