package copilotperm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
)

// PendingItem is a Copilot permission request waiting for user decision.
type PendingItem struct {
	RequestID string
	SessionID string
	Summary   string
	Kind      string
	ToolName  string
	CreatedAt time.Time
}

type pendingCopilotRequest struct {
	id     string
	item   PendingItem
	respCh chan bool
}

// Broker queues Copilot permission requests for interactive approval.
type Broker struct {
	mu      sync.Mutex
	nextID  int64
	pending []*pendingCopilotRequest
}

// NewBroker creates a Copilot permission broker.
func NewBroker() *Broker {
	return &Broker{}
}

// Prompter returns a Prompter that blocks until the request is resolved.
func (b *Broker) Prompter() Prompter {
	return func(ctx context.Context, request copilot.PermissionRequest, summary string) (bool, error) {
		if b == nil {
			return false, fmt.Errorf("copilot permission broker is nil")
		}

		req := b.enqueue(request, summary)
		select {
		case ok := <-req.respCh:
			b.remove(req.id)
			return ok, nil
		case <-ctx.Done():
			_ = b.Resolve(req.id, false)
			return false, ctx.Err()
		}
	}
}

// Pending returns unresolved Copilot permission requests.
func (b *Broker) Pending() []PendingItem {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]PendingItem, 0, len(b.pending))
	for _, req := range b.pending {
		if req == nil {
			continue
		}
		out = append(out, req.item)
	}
	return out
}

// Resolve approves or denies a pending request by ID.
func (b *Broker) Resolve(requestID string, allow bool) error {
	if b == nil {
		return fmt.Errorf("copilot permission broker is nil")
	}
	req := b.find(strings.TrimSpace(requestID))
	if req == nil {
		return fmt.Errorf("copilot request not found: %s", requestID)
	}
	select {
	case req.respCh <- allow:
		return nil
	default:
		return fmt.Errorf("copilot request already resolved: %s", requestID)
	}
}

// ResolveLatest resolves the most recent pending request.
func (b *Broker) ResolveLatest(allow bool) error {
	if b == nil {
		return fmt.Errorf("copilot permission broker is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return fmt.Errorf("no pending copilot permission requests")
	}
	req := b.pending[len(b.pending)-1]
	select {
	case req.respCh <- allow:
		return nil
	default:
		return fmt.Errorf("copilot request already resolved: %s", req.id)
	}
}

func (b *Broker) enqueue(request copilot.PermissionRequest, summary string) *pendingCopilotRequest {
	req := &pendingCopilotRequest{
		id:     b.nextRequestID(),
		respCh: make(chan bool, 1),
		item: PendingItem{
			RequestID: "",
			SessionID: "",
			Summary:   strings.TrimSpace(summary),
			Kind:      string(request.Kind),
			ToolName:  deref(request.ToolName),
			CreatedAt: time.Now().UTC(),
		},
	}
	req.item.RequestID = req.id

	b.mu.Lock()
	b.pending = append(b.pending, req)
	b.mu.Unlock()
	return req
}

func (b *Broker) nextRequestID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	return fmt.Sprintf("cperm_%d", b.nextID)
}

func (b *Broker) find(requestID string) *pendingCopilotRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, req := range b.pending {
		if req != nil && req.id == requestID {
			return req
		}
	}
	return nil
}

func (b *Broker) remove(requestID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	filtered := make([]*pendingCopilotRequest, 0, len(b.pending))
	for _, req := range b.pending {
		if req == nil || req.id == requestID {
			continue
		}
		filtered = append(filtered, req)
	}
	b.pending = filtered
}

// ParseCopilotRequestIndex resolves 1-based index to request ID.
func ParseCopilotRequestIndex(pending []PendingItem, oneBased int) (string, error) {
	if oneBased <= 0 || oneBased > len(pending) {
		return "", fmt.Errorf("copilot permission index out of range: %d", oneBased)
	}
	return pending[oneBased-1].RequestID, nil
}

// ParseCopilotRequestArg resolves arg as cperm_ID or numeric index.
func ParseCopilotRequestArg(pending []PendingItem, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		if len(pending) == 0 {
			return "", fmt.Errorf("no pending copilot permission requests")
		}
		return pending[len(pending)-1].RequestID, nil
	}
	if strings.HasPrefix(arg, "cperm_") {
		return arg, nil
	}
	if idx, err := strconv.Atoi(arg); err == nil {
		return ParseCopilotRequestIndex(pending, idx)
	}
	return "", fmt.Errorf("unknown copilot permission selector: %s", arg)
}
