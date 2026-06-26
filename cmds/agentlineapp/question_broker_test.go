package agentlineapp

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestQuestionBroker_RequestAndReply(t *testing.T) {
	b := NewQuestionBroker()
	respCh := make(chan AskResponse, 1)

	go func() {
		resp, _ := b.Request(context.Background(), AskRequest{Prompt: "继续吗？"})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(b.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(b.Pending()) == 0 {
		t.Fatal("expected pending question")
	}
	if err := b.Reply("", "继续"); err != nil {
		t.Fatalf("reply failed: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Cancelled || strings.TrimSpace(resp.Answer) != "继续" {
			t.Fatalf("unexpected ask response: %+v", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting question response")
	}
}

func TestHandleSlashInput_QuestionsAndReply(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan AskResponse, 1)
	go func() {
		resp, _ := m.questionBroker.Request(context.Background(), AskRequest{Prompt: "请输入确认"})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.questionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	handled, cmd := m.handleSlashInput("/questions")
	if !handled || cmd != nil {
		t.Fatalf("expected /questions handled")
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Title != "/questions" {
		t.Fatalf("expected /questions block title, got %q", last.Title)
	}

	handled, cmd = m.handleSlashInput("/reply 已确认")
	if !handled || cmd != nil {
		t.Fatalf("expected /reply handled")
	}

	select {
	case resp := <-respCh:
		if strings.TrimSpace(resp.Answer) != "已确认" {
			t.Fatalf("unexpected answer: %q", resp.Answer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting reply")
	}

	// 额外校验：运行中也允许 /questions 与 /reply
	m.running = true
	m.input.SetValue("/questions")
	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if m.blocks[len(m.blocks)-1].Title != "/questions" {
		t.Fatalf("expected /questions allowed while running")
	}
}

func TestHandleSlashInput_ReplyByIndex(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan AskResponse, 1)
	go func() {
		resp, _ := m.questionBroker.Request(context.Background(), AskRequest{Prompt: "请输入确认"})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.questionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	handled, cmd := m.handleSlashInput("/reply 1 好的")
	if !handled || cmd != nil {
		t.Fatalf("expected /reply handled")
	}

	select {
	case resp := <-respCh:
		if strings.TrimSpace(resp.Answer) != "好的" {
			t.Fatalf("unexpected answer: %q", resp.Answer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting reply")
	}
}

func TestRunning_DirectInputAnswersPendingQuestion(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan AskResponse, 1)
	go func() {
		resp, _ := m.questionBroker.Request(context.Background(), AskRequest{Prompt: "请输入确认"})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.questionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	m.running = true
	m.input.SetValue("直接回复")
	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)

	if len(m.blocks) == 0 || m.blocks[len(m.blocks)-1].Title != "reply.direct" {
		t.Fatalf("expected reply.direct block after direct input while running")
	}

	select {
	case resp := <-respCh:
		if strings.TrimSpace(resp.Answer) != "直接回复" {
			t.Fatalf("unexpected answer: %q", resp.Answer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting direct reply")
	}
}
