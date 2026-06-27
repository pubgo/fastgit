package copilotperm

import "strings"

// InjectPermissionMode prepends --permission-mode when the flag is not already set.
func InjectPermissionMode(args []string, mode Mode) []string {
	if mode == "" || HasPermissionModeFlag(args) {
		return args
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, "--permission-mode", string(mode))
	out = append(out, args...)
	return out
}

// HasPermissionModeFlag reports whether argv already contains permission-mode.
func HasPermissionModeFlag(args []string) bool {
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "--permission-mode" {
			return true
		}
		if strings.HasPrefix(arg, "--permission-mode=") {
			return true
		}
	}
	return false
}
