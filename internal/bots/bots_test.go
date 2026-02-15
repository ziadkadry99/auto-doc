package bots

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockHandler implements MessageHandler for testing.
type mockHandler struct {
	lastMsg  IncomingMessage
	response *OutgoingMessage
	err      error
}

func (m *mockHandler) HandleMessage(_ context.Context, msg IncomingMessage) (*OutgoingMessage, error) {
	m.lastMsg = msg
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	return &OutgoingMessage{
		ChannelID: msg.ChannelID,
		ThreadID:  msg.ThreadID,
		Text:      "mock response",
	}, nil
}

// --- Processor intent detection tests ---

func TestProcessorIntentAsk(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "ask what is the auth service?",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	// With nil engine, the response should contain an error message.
	if !strings.Contains(resp.Text, "context engine not configured") {
		t.Errorf("expected error about context engine, got: %s", resp.Text)
	}
}

func TestProcessorIntentAskQuestionMark(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "?what is the auth service",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "context engine not configured") {
		t.Errorf("expected error about context engine, got: %s", resp.Text)
	}
}

func TestProcessorIntentContext(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "context the auth service uses OAuth2",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "context engine not configured") {
		t.Errorf("expected error about context engine, got: %s", resp.Text)
	}
}

func TestProcessorIntentInfo(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "info the payment service handles Stripe",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "context engine not configured") {
		t.Errorf("expected error about context engine, got: %s", resp.Text)
	}
}

func TestProcessorIntentBacklog(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "backlog",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "backlog store not configured") {
		t.Errorf("expected error about backlog store, got: %s", resp.Text)
	}
}

func TestProcessorIntentQuestions(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "questions",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "backlog store not configured") {
		t.Errorf("expected error about backlog store, got: %s", resp.Text)
	}
}

func TestProcessorIntentDefault(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "the auth service uses OAuth2",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	// Default intent is context provision, which needs engine.
	if !strings.Contains(resp.Text, "context engine not configured") {
		t.Errorf("expected error about context engine, got: %s", resp.Text)
	}
}

func TestProcessorEmptyMessage(t *testing.T) {
	p := NewProcessor(nil, nil)
	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "",
	}
	resp, err := p.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "empty message") {
		t.Errorf("expected empty message response, got: %s", resp.Text)
	}
}

// --- Gateway tests ---

func TestGatewayProcess(t *testing.T) {
	mock := &mockHandler{
		response: &OutgoingMessage{
			ChannelID: "C123",
			Text:      "hello from mock",
		},
	}
	gw := NewGateway(mock)

	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		UserID:    "U456",
		Text:      "hello",
	}
	resp, err := gw.Process(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello from mock" {
		t.Errorf("expected 'hello from mock', got %q", resp.Text)
	}
	if mock.lastMsg.Text != "hello" {
		t.Errorf("handler did not receive message, got text: %q", mock.lastMsg.Text)
	}
}

func TestGatewayProcessError(t *testing.T) {
	mock := &mockHandler{
		err: fmt.Errorf("handler failure"),
	}
	gw := NewGateway(mock)

	msg := IncomingMessage{
		Platform:  PlatformSlack,
		ChannelID: "C123",
		Text:      "hello",
	}
	_, err := gw.Process(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "handler failure") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Slack handler tests ---

func TestSlackURLVerification(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "")

	payload := `{"type":"url_verification","challenge":"test-challenge-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["challenge"] != "test-challenge-123" {
		t.Errorf("expected challenge 'test-challenge-123', got %q", resp["challenge"])
	}
}

func TestSlackMessageEvent(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "")

	payload := `{
		"type": "event_callback",
		"event": {
			"type": "message",
			"user": "U123",
			"text": "ask about the system",
			"channel": "C456",
			"ts": "1234567890.123456",
			"thread_ts": "1234567890.000000"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if mock.lastMsg.Platform != PlatformSlack {
		t.Errorf("expected platform slack, got %s", mock.lastMsg.Platform)
	}
	if mock.lastMsg.ChannelID != "C456" {
		t.Errorf("expected channel C456, got %s", mock.lastMsg.ChannelID)
	}
	if mock.lastMsg.UserID != "U123" {
		t.Errorf("expected user U123, got %s", mock.lastMsg.UserID)
	}
	if mock.lastMsg.Text != "ask about the system" {
		t.Errorf("expected text 'ask about the system', got %q", mock.lastMsg.Text)
	}
	if mock.lastMsg.ThreadID != "1234567890.000000" {
		t.Errorf("expected thread_ts, got %q", mock.lastMsg.ThreadID)
	}
}

func TestSlackBotMessageSkipped(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "")

	payload := `{
		"type": "event_callback",
		"event": {
			"type": "message",
			"bot_id": "B123",
			"text": "I am a bot",
			"channel": "C456"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// The mock handler should not have been called.
	if mock.lastMsg.Text != "" {
		t.Errorf("bot message should have been skipped, but handler received: %q", mock.lastMsg.Text)
	}
}

func TestSlackNonMessageEventSkipped(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "")

	payload := `{
		"type": "event_callback",
		"event": {
			"type": "reaction_added",
			"user": "U123",
			"channel": "C456"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if mock.lastMsg.Text != "" {
		t.Errorf("non-message event should have been skipped")
	}
}

func TestSlackInvalidJSON(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "")

	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSlackSignatureVerification(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewSlackHandler(gw, "test-secret")

	payload := `{"type":"url_verification","challenge":"c"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/slack/events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	// No signature headers -> should fail.
	w := httptest.NewRecorder()

	handler.HandleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Teams handler tests ---

func TestTeamsMessageActivity(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewTeamsHandler(gw)

	payload := `{
		"type": "message",
		"id": "activity-1",
		"timestamp": "2024-01-15T12:00:00Z",
		"text": "tell me about the system",
		"from": {"id": "user-1", "name": "Alice"},
		"conversation": {"id": "conv-1"},
		"channelId": "msteams",
		"replyToId": "parent-1"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/teams/activity", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleActivity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if mock.lastMsg.Platform != PlatformTeams {
		t.Errorf("expected platform teams, got %s", mock.lastMsg.Platform)
	}
	if mock.lastMsg.ChannelID != "conv-1" {
		t.Errorf("expected channel conv-1, got %s", mock.lastMsg.ChannelID)
	}
	if mock.lastMsg.UserID != "user-1" {
		t.Errorf("expected user user-1, got %s", mock.lastMsg.UserID)
	}
	if mock.lastMsg.UserName != "Alice" {
		t.Errorf("expected username Alice, got %s", mock.lastMsg.UserName)
	}
	if mock.lastMsg.Text != "tell me about the system" {
		t.Errorf("expected text, got %q", mock.lastMsg.Text)
	}
	if mock.lastMsg.ThreadID != "parent-1" {
		t.Errorf("expected thread parent-1, got %s", mock.lastMsg.ThreadID)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["type"] != "message" {
		t.Errorf("expected response type 'message', got %q", resp["type"])
	}
}

func TestTeamsNonMessageActivitySkipped(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewTeamsHandler(gw)

	payload := `{
		"type": "conversationUpdate",
		"from": {"id": "user-1", "name": "Alice"},
		"conversation": {"id": "conv-1"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/bots/teams/activity", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleActivity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if mock.lastMsg.Text != "" {
		t.Errorf("non-message activity should have been skipped")
	}
}

func TestTeamsInvalidJSON(t *testing.T) {
	mock := &mockHandler{}
	gw := NewGateway(mock)
	handler := NewTeamsHandler(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/bots/teams/activity", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleActivity(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Slack formatting test ---

func TestFormatSlackMessage(t *testing.T) {
	msg := &OutgoingMessage{
		ChannelID: "C123",
		Text:      "- item 1\n- item 2\nplain line",
		ThreadID:  "ts-1",
	}
	resp := formatSlackMessage(msg)
	if resp.Channel != "C123" {
		t.Errorf("expected channel C123, got %s", resp.Channel)
	}
	if resp.ThreadTS != "ts-1" {
		t.Errorf("expected thread_ts ts-1, got %s", resp.ThreadTS)
	}
	if !strings.Contains(resp.Text, "•") {
		t.Errorf("expected bullet formatting, got %q", resp.Text)
	}
}

func TestFormatSlackMessageNoThread(t *testing.T) {
	msg := &OutgoingMessage{
		ChannelID: "C123",
		Text:      "simple response",
	}
	resp := formatSlackMessage(msg)
	if resp.ThreadTS != "" {
		t.Errorf("expected empty thread_ts, got %s", resp.ThreadTS)
	}
}
