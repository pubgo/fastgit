package agentlineapp

import (
	"context"
	"strings"
	"testing"

	"github.com/pubgo/fastgit/pkg/copilotperm"
	"github.com/pubgo/redant"
)

func TestPermissionModeSlashShowsCurrent(t *testing.T) {
	root := &redant.Command{Use: "copilot"}
	m := newAgentlineModel(context.Background(), root, "copilot> ", nil, "", false, nil, "deny")

	handled, cmd := m.handleSlashInput("/permission-mode")
	if !handled || cmd != nil {
		t.Fatalf("expected /permission-mode handled")
	}
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(strings.Join(last.Lines, "\n"), "deny") {
		t.Fatalf("expected current mode deny, got %q", last.Lines)
	}
}

func TestPermissionModeSlashChangesMode(t *testing.T) {
	root := &redant.Command{Use: "copilot"}
	m := newAgentlineModel(context.Background(), root, "copilot> ", nil, "", false, nil, "")

	handled, cmd := m.handleSlashInput("/permission-mode allow")
	if !handled || cmd != nil {
		t.Fatalf("expected /permission-mode allow handled")
	}
	if m.permissionMode != copilotperm.ModeAllow {
		t.Fatalf("expected allow, got %s", m.permissionMode)
	}
}

func TestInjectPermissionModeInRunSlashRunCmd(t *testing.T) {
	root := &redant.Command{Use: "copilot"}
	m := newAgentlineModel(context.Background(), root, "copilot> ", nil, "", false, nil, "ask")
	m.permissionMode = copilotperm.ModeDeny

	args := copilotperm.InjectPermissionMode([]string{"chat", "--prompt", "hi"}, m.permissionMode)
	if !copilotperm.HasPermissionModeFlag(args) {
		t.Fatalf("expected injected permission flag, got %v", args)
	}
	if args[0] != "--permission-mode" || args[1] != "deny" {
		t.Fatalf("unexpected injected args: %v", args)
	}
}
