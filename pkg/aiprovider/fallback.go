package aiprovider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var diffFilePattern = regexp.MustCompile(`(?m)^diff --git a/(.+?) b/`)

// RuleFallback generates deterministic text when no AI provider is available.
type RuleFallback struct{}

func NewRuleFallback() *RuleFallback { return &RuleFallback{} }

func (p *RuleFallback) Name() string { return "rule-fallback" }

func (p *RuleFallback) Available() bool { return true }

func (p *RuleFallback) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	_ = ctx
	text := CommitMessageFromDiff(req.User)
	return CompleteResponse{
		Text:     text,
		Provider: p.Name(),
		Fallback: true,
	}, nil
}

// CommitMessageFromDiff builds a conventional-style message from a git diff.
func CommitMessageFromDiff(diff string) string {
	files := filesFromDiff(diff)
	switch len(files) {
	case 0:
		return "chore: update changes"
	case 1:
		return fmt.Sprintf("chore: update %s", trimPath(files[0]))
	default:
		return fmt.Sprintf("chore: update %d files", len(files))
	}
}

func filesFromDiff(diff string) []string {
	matches := diffFilePattern.FindAllStringSubmatch(diff, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var files []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		file := strings.TrimSpace(match[1])
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		files = append(files, file)
	}
	return files
}

func trimPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "changes"
	}
	if len(path) > 48 {
		return "..." + path[len(path)-45:]
	}
	return path
}
