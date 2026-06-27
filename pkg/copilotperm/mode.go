package copilotperm

import (
	"fmt"
	"strings"
)

// Mode controls how Copilot permission requests are handled.
type Mode string

const (
	ModeAsk   Mode = "ask"
	ModeAllow Mode = "allow"
	ModeDeny  Mode = "deny"
)

// DefaultMode is the safe default for interactive Copilot sessions.
const DefaultMode = ModeAsk

// ParseMode normalizes and validates a permission mode string.
func ParseMode(raw string) (Mode, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(raw)))
	switch mode {
	case ModeAsk, ModeAllow, ModeDeny, "":
		if mode == "" {
			return DefaultMode, nil
		}
		return mode, nil
	default:
		return "", fmt.Errorf("invalid permission mode %q (want ask|allow|deny)", raw)
	}
}
