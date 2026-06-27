package gitconflict

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
)

const conflictAISystemPrompt = `You analyze git merge conflict markers in file excerpts.
For each FILE block, reply with exactly one line:
REASON: <why the conflict likely happened and what to verify>

Be concise. Do not invent files not shown.`

// EnhanceSnapshot optionally replaces heuristic reasons with AI analysis.
func EnhanceSnapshot(ctx context.Context, snap Snapshot, provider aiprovider.Provider, repoRoot string) (Snapshot, error) {
	if len(snap.Files) == 0 || provider == nil || !provider.Available() {
		return snap, nil
	}

	userPrompt, err := buildConflictPrompt(snap.Files, repoRoot)
	if err != nil {
		return snap, err
	}
	if strings.TrimSpace(userPrompt) == "" {
		return snap, nil
	}

	resp, err := provider.Complete(ctx, aiprovider.CompleteRequest{
		System: conflictAISystemPrompt,
		User:   userPrompt,
	})
	if err != nil {
		return snap, err
	}
	applyAIReasons(&snap, resp.Text)
	return snap, nil
}

func buildConflictPrompt(files []File, repoRoot string) (string, error) {
	var b strings.Builder
	for _, file := range files {
		path := filepath.Join(repoRoot, file.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read conflict file %s: %w", file.Path, err)
		}
		regions := extractConflictRegions(string(data))
		if regions == "" {
			continue
		}
		b.WriteString("FILE: ")
		b.WriteString(file.Path)
		b.WriteByte('\n')
		b.WriteString(regions)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func extractConflictRegions(content string) string {
	const maxLen = 4000
	lines := strings.Split(content, "\n")
	var b strings.Builder
	inConflict := false
	for _, line := range lines {
		if strings.HasPrefix(line, "<<<<<<<") {
			inConflict = true
		}
		if inConflict {
			b.WriteString(line)
			b.WriteByte('\n')
			if b.Len() > maxLen {
				b.WriteString("... (truncated)\n")
				break
			}
		}
		if strings.HasPrefix(line, ">>>>>>>") {
			inConflict = false
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func applyAIReasons(snap *Snapshot, text string) {
	if snap == nil {
		return
	}
	reasons := parseAIReasons(text)
	if len(reasons) == 0 {
		return
	}

	for i := range snap.Files {
		if reason, ok := reasons[snap.Files[i].Path]; ok && strings.TrimSpace(reason) != "" {
			snap.Files[i].Reason = reason
		}
	}

	groups := map[string][]string{}
	for _, file := range snap.Files {
		groups[file.Module] = append(groups[file.Module], file.Path)
	}
	snap.Groups = groups
	snap.Summary = renderSummary(snap.Files, snap.Groups)
}

func parseAIReasons(text string) map[string]string {
	out := map[string]string{}
	current := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FILE:") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "FILE:"))
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "REASON:") {
			reason := strings.TrimSpace(line[strings.Index(line, ":")+1:])
			if current != "" && reason != "" {
				out[current] = reason
			}
		}
	}
	return out
}

// BuildSnapshotWithAI builds a snapshot and optionally enhances it with AI reasons.
func BuildSnapshotWithAI(ctx context.Context, repoRoot string, provider aiprovider.Provider, useAI bool) (Snapshot, error) {
	snap, err := BuildSnapshot(ctx, repoRoot)
	if err != nil {
		return Snapshot{}, err
	}
	if !useAI {
		return snap, nil
	}
	return EnhanceSnapshot(ctx, snap, provider, repoRoot)
}
