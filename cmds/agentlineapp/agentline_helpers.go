package agentlineapp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pubgo/redant/internal/gitshell"
)

func runStatus(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "canceled"
	}
	return "failed"
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	return d.Round(time.Millisecond).String()
}

func loadHistory(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	out := make([]string, 0)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func appendHistoryLine(path, line string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(line) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintln(f, line)
	return err
}

func splitCommandLine(input string) ([]string, error) {
	var (
		out     []string
		cur     strings.Builder
		quote   rune
		escaped bool
	)
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}

	for _, r := range input {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}

	if escaped {
		return nil, errors.New("unfinished escape sequence")
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote")
	}
	flush()
	return out, nil
}

func formatCommandLine(program string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteShellArg(program))
	for _, arg := range args {
		parts = append(parts, quoteShellArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteShellArg(s string) string {
	if s == "" {
		return `""`
	}
	if !needsQuote(s) {
		return s
	}
	return strconv.Quote(s)
}

func (m *agentlineModel) sessionContextLine() string {
	return fmt.Sprintf("cwd=%s · git=%s", displayPath(m.sessionCWD), displayGitBranch(m.sessionGitBranch, m.sessionGitDirty))
}

func (m *agentlineModel) chatBindingLine() string {
	if m == nil || !m.isChatMode() || m.stickyInvocation == nil {
		return ""
	}

	parts := append([]string(nil), m.stickyInvocation.BaseArgs...)
	parts = append(parts, m.stickyInvocation.PromptFlag, "<text>")
	return fmt.Sprintf("chat=%s", strings.Join(parts, " "))
}

func displayPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "(unknown)"
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		home = filepath.Clean(home)
		cleanPath := filepath.Clean(path)
		if cleanPath == home {
			return "~"
		}
		prefix := home + string(os.PathSeparator)
		if strings.HasPrefix(cleanPath, prefix) {
			return "~" + string(os.PathSeparator) + strings.TrimPrefix(cleanPath, prefix)
		}
	}
	return path
}

func displayGitBranch(branch string, dirty bool) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "(not repo)"
	}
	if dirty {
		return branch + "*"
	}
	return branch
}

func detectSessionContext() (cwd, gitBranch string, gitDirty bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", false
	}
	return wd, gitshell.DetectBranch(wd), gitshell.IsDirty(wd)
}

func needsQuote(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
		switch r {
		case '"', '\'', '\\', '$', '`', '|', '&', ';', '(', ')', '<', '>', '*', '?', '[', ']', '{', '}', '!':
			return true
		}
	}
	return false
}
