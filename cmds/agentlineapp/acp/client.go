package agentacp

import (
	"context"

	acp "github.com/coder/acp-go-sdk"
)

// CallbackClient 提供一个可插拔的 ACP Client 侧适配器。
//
// 说明：
// - 优先满足 session/update 与 session/request_permission；
// - 其余 fs/terminal 方法按需挂接；
// - 未提供回调时返回 MethodNotFound（或默认取消权限请求）。
type CallbackClient struct {
	PermissionBroker    *PermissionBroker
	OnReadTextFile      func(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error)
	OnWriteTextFile     func(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error)
	OnRequestPermission func(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
	OnSessionUpdate     func(ctx context.Context, params acp.SessionNotification) error
	OnCreateTerminal    func(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error)
	OnKillTerminal      func(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error)
	OnTerminalOutput    func(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error)
	OnReleaseTerminal   func(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error)
	OnWaitTerminalExit  func(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error)
}

func (c *CallbackClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if c != nil && c.OnReadTextFile != nil {
		return c.OnReadTextFile(ctx, params)
	}
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}

func (c *CallbackClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if c != nil && c.OnWriteTextFile != nil {
		return c.OnWriteTextFile(ctx, params)
	}
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}

func (c *CallbackClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if c != nil && c.OnRequestPermission != nil {
		return c.OnRequestPermission(ctx, params)
	}
	if c != nil && c.PermissionBroker != nil {
		return c.PermissionBroker.RequestPermission(ctx, params)
	}
	// 默认行为：在无 UI 回调时返回 cancelled，避免卡住 prompt 流程。
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

func (c *CallbackClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	if c != nil && c.OnSessionUpdate != nil {
		return c.OnSessionUpdate(ctx, params)
	}
	return nil
}

func (c *CallbackClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	if c != nil && c.OnCreateTerminal != nil {
		return c.OnCreateTerminal(ctx, params)
	}
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (c *CallbackClient) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	if c != nil && c.OnKillTerminal != nil {
		return c.OnKillTerminal(ctx, params)
	}
	return acp.KillTerminalCommandResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

func (c *CallbackClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	if c != nil && c.OnTerminalOutput != nil {
		return c.OnTerminalOutput(ctx, params)
	}
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (c *CallbackClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	if c != nil && c.OnReleaseTerminal != nil {
		return c.OnReleaseTerminal(ctx, params)
	}
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (c *CallbackClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	if c != nil && c.OnWaitTerminalExit != nil {
		return c.OnWaitTerminalExit(ctx, params)
	}
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}
