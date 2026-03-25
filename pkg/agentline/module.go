package agentline

import "strings"

const (
	// CommandMetaMode is the metadata key used to mark command mode.
	CommandMetaMode = "mode"
	// CommandMetaModeAgent indicates the command should run in agent mode.
	CommandMetaModeAgent = "agent"
	// CommandMetaAgentCommand marks a command as an agent command.
	CommandMetaAgentCommand = "agent.command"
	// CommandMetaAgentEntry marks a command as an agent entry command.
	CommandMetaAgentEntry = "agent.entry"

	// CommandName is the built-in agentline command name.
	CommandName = "agentline"
	// InitialArgKey is the hidden argument key used for bootstrap argv injection.
	InitialArgKey = "initial-arg"
)

// AgentCommandMetadata returns metadata marking a command as an agent command.
func AgentCommandMetadata() map[string]string {
	return map[string]string{CommandMetaAgentCommand: "true"}
}

// AgentEntryMetadata returns metadata marking a command as an agent entry.
func AgentEntryMetadata() map[string]string {
	return map[string]string{CommandMetaAgentEntry: "true"}
}

// Meta returns metadata value by key. Key lookup is case-insensitive.
func Meta(metadata map[string]string, key string) string {
	if strings.TrimSpace(key) == "" || len(metadata) == 0 {
		return ""
	}

	if v, ok := metadata[key]; ok {
		return strings.TrimSpace(v)
	}

	for k, v := range metadata {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v)
		}
	}

	return ""
}

// IsAgentCommand reports whether metadata marks a command as agent command.
func IsAgentCommand(metadata map[string]string) bool {
	if IsAgentEntryCommand(metadata) {
		return true
	}
	if metaTruthy(Meta(metadata, CommandMetaAgentCommand)) {
		return true
	}
	return false
}

// IsAgentEntryCommand reports whether metadata marks a command as an
// interactive entry that should auto-route to agentline.
func IsAgentEntryCommand(metadata map[string]string) bool {
	if strings.EqualFold(Meta(metadata, CommandMetaMode), CommandMetaModeAgent) {
		return true
	}
	return metaTruthy(Meta(metadata, CommandMetaAgentEntry))
}

func metaTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// BuildBootstrapArgs converts raw argv into repeated hidden-flag format.
func BuildBootstrapArgs(initialArgv []string) []string {
	if len(initialArgv) == 0 {
		return nil
	}

	out := make([]string, 0, len(initialArgv)*2)
	for _, arg := range initialArgv {
		out = append(out, "--"+InitialArgKey, arg)
	}
	return out
}
