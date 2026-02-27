package historycmd

import (
	"context"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/redant"
	"github.com/samber/lo"
)

var path = "/Users/barry/Documents/git/zshrc.123.history"

func New() *redant.Command {
	return &redant.Command{
		Use:   "history",
		Short: "shell history command management",
		Children: []*redant.Command{
			{
				Use: "rewrite",
				Handler: func(ctx context.Context, command *redant.Invocation) error {
					assert.Exit(utils.ShellExec(ctx, "cat", "~/.zsh_history", ">>", path))
					var data = lo.Must(os.ReadFile(path))
					var set = mapset.NewSet[string]()
					for _, line := range strings.Split(string(data), "\n") {
						lines := strings.SplitN(strings.TrimSpace(line), " ", 2)
						if len(lines) == 2 && isNumber(strings.Trim(strings.TrimSpace(lines[0]), "*")) {
							line = strings.TrimSpace(lines[1])
						}
						line = strings.TrimSpace(line)
						set.Add(line)
					}
					return os.WriteFile(path, []byte(strings.Join(set.ToSlice(), "\n")), 0644)
				},
			},
		},
		Handler: func(ctx context.Context, command *redant.Invocation) error {
			defer recovery.Exit()

			var data = lo.Must(os.ReadFile(path))
			var set = mapset.NewSet[string]()
			for _, line := range strings.Split(string(data), "\n") {
				lines := strings.SplitN(strings.TrimSpace(line), " ", 2)
				if len(lines) == 2 && isNumber(strings.Trim(strings.TrimSpace(lines[0]), "*")) {
					line = strings.TrimSpace(lines[1])
				}
				line = strings.TrimSpace(line)
				set.Add(line)
			}

			p := tea.NewProgram(initialModel(set.ToSlice()))
			_ = lo.Must(p.Run())
			return nil
		},
	}
}

func isNumber(str string) bool {
	if str == "" {
		return false
	}

	_, err := strconv.Atoi(str)
	return err == nil
}
