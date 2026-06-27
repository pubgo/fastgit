package workflow

import (
	"fmt"
	"io"
)

// PrintRecommendations records a command and prints next-step hints.
func PrintRecommendations(out io.Writer, command string) {
	mem, err := NewMemory()
	if err != nil {
		return
	}
	_ = mem.Record(command)
	recs := mem.Recommend(command)
	if hint := FormatHint(command, recs); hint != "" {
		_, _ = fmt.Fprintln(out, hint)
	}
}
