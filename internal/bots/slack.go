package bots

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SlackHandler handles incoming Slack webhook events.
type SlackHandler struct {
	gateway       *Gateway
	signingSecret string
}

// NewSlackHandler creates a new Slack event handler.
func NewSlackHandler(gateway *Gateway, signingSecret string) *SlackHandler {
	return &SlackHandler{
		gateway:       gateway,
		signingSecret: signingSecret,
	}
}

// slackEvent represents the top-level Slack event payload.
type slackEvent struct {
	Type      string          `json:"type"`
	Token     string          `json:"token"`
	Challenge string          `json:"challenge"`
	Event     slackInnerEvent `json:"event"`
}

// slackInnerEvent represents the inner event in a Slack event_callback.
type slackInnerEvent struct {
	Type    string `json:"type"`
	User    string `json:"user"`
	Text    string `json:"text"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
	BotID   string `json:"bot_id"`
}

// HandleEvent handles incoming Slack events (HTTP POST).
func (h *SlackHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify Slack request signature if signing secret is configured.
	if h.signingSecret != "" {
		if !h.verifySignature(r, body) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var event slackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "url_verification":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": event.Challenge})
		return

	case "event_callback":
		// Skip bot messages to avoid loops.
		if event.Event.BotID != "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Only handle message events.
		if event.Event.Type != "message" {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := IncomingMessage{
			Platform:  PlatformSlack,
			ChannelID: event.Event.Channel,
			UserID:    event.Event.User,
			Text:      event.Event.Text,
			ThreadID:  event.Event.ThreadTS,
			Timestamp: event.Event.TS,
		}

		resp, err := h.gateway.Process(r.Context(), msg)
		if err != nil {
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return

	default:
		w.WriteHeader(http.StatusOK)
	}
}

// verifySignature verifies the Slack request signature using HMAC-SHA256.
func (h *SlackHandler) verifySignature(r *http.Request, body []byte) bool {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return false
	}

	// Check that the timestamp is not too old (5 minutes).
	ts, err := fmt.Sscanf(timestamp, "%d", new(int64))
	if err != nil || ts == 0 {
		return false
	}

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(h.signingSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// verifyTimestamp checks that the request timestamp is within 5 minutes.
func verifyTimestamp(timestamp string) bool {
	var ts int64
	if _, err := fmt.Sscanf(timestamp, "%d", &ts); err != nil {
		return false
	}
	diff := time.Now().Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	return diff <= 300
}

// slackResponse represents a simple Slack response message.
type slackResponse struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

// formatSlackMessage creates a Slack-formatted response payload.
func formatSlackMessage(msg *OutgoingMessage) *slackResponse {
	resp := &slackResponse{
		Channel: msg.ChannelID,
		Text:    msg.Text,
	}
	if msg.ThreadID != "" {
		resp.ThreadTS = msg.ThreadID
	}

	// Wrap multi-line responses with basic formatting.
	if strings.Contains(resp.Text, "\n") {
		lines := strings.Split(resp.Text, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "- ") {
				lines[i] = "• " + line[2:]
			}
		}
		resp.Text = strings.Join(lines, "\n")
	}

	return resp
}
