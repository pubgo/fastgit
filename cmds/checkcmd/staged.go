package checkcmd

import (
	"path/filepath"
	"sort"
	"strings"
)

func stagedPackagePatterns(stagedFiles []string) []string {
	patterns := map[string]struct{}{}
	for _, file := range stagedFiles {
		file = filepath.ToSlash(strings.TrimSpace(file))
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		parts := strings.Split(file, "/")
		switch {
		case len(parts) >= 2 && (parts[0] == "pkg" || parts[0] == "cmds" || parts[0] == "internal"):
			patterns["./"+parts[0]+"/"+parts[1]+"/..."] = struct{}{}
		case len(parts) == 1:
			patterns["./..."] = struct{}{}
		default:
			patterns["./"+parts[0]+"/..."] = struct{}{}
		}
	}
	out := make([]string, 0, len(patterns))
	for pattern := range patterns {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func stagedVetCommand(stagedFiles []string) string {
	patterns := stagedPackagePatterns(stagedFiles)
	if len(patterns) == 0 {
		return ""
	}
	return "go vet " + strings.Join(patterns, " ")
}

func stagedTestCommand(stagedFiles []string) string {
	patterns := stagedPackagePatterns(stagedFiles)
	if len(patterns) == 0 {
		return ""
	}
	return "go test -short " + strings.Join(patterns, " ")
}
