package agentlineapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/redant"
)

// InteractionAnnotationKey 是注入到 Invocation.Annotations 的协议入口键。
//
// 协议约定（v1）：
//  1. 命令侧通过 InteractionFromInvocation 获取桥接对象；
//  2. 调用 Emit 推送中间态事件（system/user/assistant/tool/error）；
//  3. 调用 Ask 发起阻塞提问，用户可在 agentline 中通过 /questions /reply /skip 响应；
//  4. Ask 返回后命令继续执行，形成“命令 <-> UI”的双向交互。
const InteractionAnnotationKey = "agentline.interaction.v1"

type InteractionEvent struct {
	Kind  string
	Title string
	Lines []string
}

type AskRequest struct {
	Prompt string
}

type AskResponse struct {
	Answer    string
	Cancelled bool
}

// InteractionBridge 定义命令与 agentline 的双向通信接口。
type InteractionBridge interface {
	Emit(ctx context.Context, event InteractionEvent) error
	Ask(ctx context.Context, req AskRequest) (AskResponse, error)
}

type runtimeInteractionBridge struct {
	emitFn func(ctx context.Context, event InteractionEvent) error
	askFn  func(ctx context.Context, req AskRequest) (AskResponse, error)
}

func (b *runtimeInteractionBridge) Emit(ctx context.Context, event InteractionEvent) error {
	if b == nil || b.emitFn == nil {
		return nil
	}
	return b.emitFn(ctx, event)
}

func (b *runtimeInteractionBridge) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	if b == nil || b.askFn == nil {
		return AskResponse{}, fmt.Errorf("interaction ask is not available")
	}
	return b.askFn(ctx, req)
}

// InteractionFromInvocation 从 Invocation 注解中提取交互桥。
func InteractionFromInvocation(inv *redant.Invocation) (InteractionBridge, bool) {
	if inv == nil || inv.Annotations == nil {
		return nil, false
	}
	v, ok := inv.Annotations[InteractionAnnotationKey]
	if !ok || v == nil {
		return nil, false
	}
	bridge, ok := v.(InteractionBridge)
	return bridge, ok
}

func interactionKindToBlockKind(kind string) blockKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "user":
		return blockKindUser
	case "assistant":
		return blockKindAssistant
	case "tool":
		return blockKindTool
	case "command":
		return blockKindCommand
	case "result":
		return blockKindResult
	case "error":
		return blockKindError
	default:
		return blockKindSystem
	}
}
