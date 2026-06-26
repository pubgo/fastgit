package agentlineapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type PendingQuestion struct {
	ID        string
	Prompt    string
	CreatedAt time.Time
}

type pendingQuestionRequest struct {
	id        string
	prompt    string
	createdAt time.Time
	respCh    chan AskResponse
}

type QuestionBroker struct {
	mu      sync.Mutex
	nextID  int64
	pending []*pendingQuestionRequest
}

func NewQuestionBroker() *QuestionBroker { return &QuestionBroker{} }

func (b *QuestionBroker) Request(ctx context.Context, req AskRequest) (AskResponse, error) {
	if b == nil {
		return AskResponse{Cancelled: true}, nil
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return AskResponse{}, errors.New("ask prompt is empty")
	}

	q := &pendingQuestionRequest{
		id:        b.nextRequestID(),
		prompt:    prompt,
		createdAt: time.Now(),
		respCh:    make(chan AskResponse, 1),
	}

	b.mu.Lock()
	b.pending = append(b.pending, q)
	b.mu.Unlock()

	select {
	case resp := <-q.respCh:
		b.remove(q.id)
		return resp, nil
	case <-ctx.Done():
		_ = b.Cancel(q.id)
		return AskResponse{Cancelled: true}, nil
	}
}

func (b *QuestionBroker) Pending() []PendingQuestion {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]PendingQuestion, 0, len(b.pending))
	for _, p := range b.pending {
		if p == nil {
			continue
		}
		out = append(out, PendingQuestion{ID: p.id, Prompt: p.prompt, CreatedAt: p.createdAt})
	}
	return out
}

func (b *QuestionBroker) Reply(id string, answer string) error {
	if b == nil {
		return errors.New("question broker is nil")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return errors.New("reply answer is empty")
	}
	req, err := b.target(id)
	if err != nil {
		return err
	}
	select {
	case req.respCh <- AskResponse{Answer: answer}:
		return nil
	default:
		return errors.New("question already resolved")
	}
}

func (b *QuestionBroker) Cancel(id string) error {
	if b == nil {
		return errors.New("question broker is nil")
	}
	req, err := b.target(id)
	if err != nil {
		return err
	}
	select {
	case req.respCh <- AskResponse{Cancelled: true}:
		return nil
	default:
		return errors.New("question already resolved")
	}
}

func (b *QuestionBroker) target(id string) (*pendingQuestionRequest, error) {
	id = strings.TrimSpace(id)
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return nil, errors.New("no pending questions")
	}
	if id == "" {
		return b.pending[len(b.pending)-1], nil
	}
	for _, p := range b.pending {
		if p != nil && p.id == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("question not found: %s", id)
}

func (b *QuestionBroker) nextRequestID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	return fmt.Sprintf("ask_%d", b.nextID)
}

func (b *QuestionBroker) remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, p := range b.pending {
		if p != nil && p.id == id {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			return
		}
	}
}
