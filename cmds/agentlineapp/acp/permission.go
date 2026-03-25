package agentacp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

// PendingPermission 表示一个待决策的权限请求快照。
type PendingPermission struct {
	RequestID  string
	SessionID  acp.SessionId
	ToolCallID acp.ToolCallId
	Title      string
	Options    []acp.PermissionOption
}

type pendingPermissionRequest struct {
	id     string
	params acp.RequestPermissionRequest
	respCh chan acp.RequestPermissionResponse
}

// PermissionBroker 负责管理 ACP 权限请求生命周期。
type PermissionBroker struct {
	mu      sync.Mutex
	nextID  int64
	pending []*pendingPermissionRequest
}

// NewPermissionBroker 创建权限 broker。
func NewPermissionBroker() *PermissionBroker {
	return &PermissionBroker{}
}

// RequestPermission 提交请求并阻塞等待决策。
func (b *PermissionBroker) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if b == nil {
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}

	req := &pendingPermissionRequest{
		id:     b.nextRequestID(),
		params: params,
		respCh: make(chan acp.RequestPermissionResponse, 1),
	}

	b.mu.Lock()
	b.pending = append(b.pending, req)
	b.mu.Unlock()

	select {
	case resp := <-req.respCh:
		b.remove(req.id)
		return resp, nil
	case <-ctx.Done():
		_ = b.ResolveCancelled(req.id)
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}
}

// Pending 返回当前待处理权限请求快照。
func (b *PermissionBroker) Pending() []PendingPermission {
	if b == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]PendingPermission, 0, len(b.pending))
	for _, req := range b.pending {
		if req == nil {
			continue
		}
		title := ""
		if req.params.ToolCall.Title != nil {
			title = strings.TrimSpace(*req.params.ToolCall.Title)
		}
		out = append(out, PendingPermission{
			RequestID:  req.id,
			SessionID:  req.params.SessionId,
			ToolCallID: req.params.ToolCall.ToolCallId,
			Title:      title,
			Options:    append([]acp.PermissionOption(nil), req.params.Options...),
		})
	}

	return out
}

// ResolveSelected 将请求决策为 selected。
func (b *PermissionBroker) ResolveSelected(requestID string, optionID acp.PermissionOptionId) error {
	if b == nil {
		return errors.New("permission broker is nil")
	}
	req := b.find(strings.TrimSpace(requestID))
	if req == nil {
		return fmt.Errorf("request not found: %s", requestID)
	}
	select {
	case req.respCh <- acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(optionID)}:
		return nil
	default:
		return errors.New("request already resolved")
	}
}

// ResolveCancelled 将请求决策为 cancelled。
func (b *PermissionBroker) ResolveCancelled(requestID string) error {
	if b == nil {
		return errors.New("permission broker is nil")
	}
	req := b.find(strings.TrimSpace(requestID))
	if req == nil {
		return fmt.Errorf("request not found: %s", requestID)
	}
	select {
	case req.respCh <- acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}:
		return nil
	default:
		return errors.New("request already resolved")
	}
}

// ResolveFirstByKind 按 option kind 自动选择第一个匹配项。
func (b *PermissionBroker) ResolveFirstByKind(requestID string, kinds ...acp.PermissionOptionKind) error {
	if b == nil {
		return errors.New("permission broker is nil")
	}
	req := b.find(strings.TrimSpace(requestID))
	if req == nil {
		return fmt.Errorf("request not found: %s", requestID)
	}

	for _, kind := range kinds {
		for _, option := range req.params.Options {
			if option.Kind == kind {
				return b.ResolveSelected(req.id, option.OptionId)
			}
		}
	}

	return fmt.Errorf("no matching option for request %s", requestID)
}

// ResolveByIndex 使用 1-based 索引选择选项。
func (b *PermissionBroker) ResolveByIndex(requestID string, oneBasedIndex int) error {
	if b == nil {
		return errors.New("permission broker is nil")
	}
	req := b.find(strings.TrimSpace(requestID))
	if req == nil {
		return fmt.Errorf("request not found: %s", requestID)
	}
	idx := oneBasedIndex - 1
	if idx < 0 || idx >= len(req.params.Options) {
		return fmt.Errorf("invalid option index: %d", oneBasedIndex)
	}
	return b.ResolveSelected(req.id, req.params.Options[idx].OptionId)
}

// ParseIndexOrOption 尝试将文本解释为索引或 optionId。
func ParseIndexOrOption(raw string) (index int, optionID acp.PermissionOptionId, isIndex bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", false
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n, "", true
	}
	return 0, acp.PermissionOptionId(raw), false
}

func (b *PermissionBroker) nextRequestID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	return fmt.Sprintf("perm_%d", b.nextID)
}

func (b *PermissionBroker) find(requestID string) *pendingPermissionRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, req := range b.pending {
		if req != nil && req.id == requestID {
			return req
		}
	}
	return nil
}

func (b *PermissionBroker) remove(requestID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, req := range b.pending {
		if req != nil && req.id == requestID {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			return
		}
	}
}
