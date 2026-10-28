package acpclient

import (
	"context"

	acp "github.com/coder/acp-go-sdk"
)

// PermissionHandler 决定如何回应 agent 的 session/request_permission。
// Options.OnPermission 为 nil 时维持自动拒绝(commit 生成等只读场景)。
type PermissionHandler func(ctx context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)

// PermissionAllowOnce 是自动放行策略:优先 allow_once,其次 allow_always,
// 都没有则取消。适合 CI/批处理的 agent 模式。
func PermissionAllowOnce(ctx context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	cancelled := acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}
	var fallback *acp.PermissionOption
	for i := range req.Options {
		switch req.Options[i].Kind {
		case acp.PermissionOptionKindAllowOnce:
			return selected(req.Options[i]), nil
		case acp.PermissionOptionKindAllowAlways:
			if fallback == nil {
				o := req.Options[i]
				fallback = &o
			}
		}
	}
	if fallback != nil {
		return selected(*fallback), nil
	}
	return cancelled, nil
}

// PermissionInteractive 用注入的 ask 回调向用户确认。ask 返回选中的
// optionId;返回空字符串视为拒绝(取消)。
func PermissionInteractive(ask func(title string, options []acp.PermissionOption) string) PermissionHandler {
	return func(ctx context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
		title := ""
		if req.ToolCall.Title != nil {
			title = *req.ToolCall.Title
		}
		if title == "" {
			title = string(req.ToolCall.ToolCallId)
		}
		if ask == nil {
			return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
		}
		if chosen := ask(title, req.Options); chosen != "" {
			for _, opt := range req.Options {
				if string(opt.OptionId) == chosen {
					return selected(opt), nil
				}
			}
		}
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}
}

func selected(option acp.PermissionOption) acp.RequestPermissionResponse {
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeSelected(option.OptionId),
	}
}
