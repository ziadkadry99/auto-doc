package bots

import (
	"encoding/json"
	"io"
	"net/http"
)

// TeamsHandler handles incoming Microsoft Teams bot activities.
type TeamsHandler struct {
	gateway *Gateway
}

// NewTeamsHandler creates a new Teams activity handler.
func NewTeamsHandler(gateway *Gateway) *TeamsHandler {
	return &TeamsHandler{gateway: gateway}
}

// teamsActivity represents a Teams Bot Framework activity.
type teamsActivity struct {
	Type         string            `json:"type"`
	ID           string            `json:"id"`
	Timestamp    string            `json:"timestamp"`
	Text         string            `json:"text"`
	From         teamsAccount      `json:"from"`
	Conversation teamsConversation `json:"conversation"`
	ChannelID    string            `json:"channelId"`
	ServiceURL   string            `json:"serviceUrl"`
	ReplyToID    string            `json:"replyToId"`
}

// teamsAccount represents a user or bot account in Teams.
type teamsAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// teamsConversation identifies the Teams conversation.
type teamsConversation struct {
	ID string `json:"id"`
}

// HandleActivity handles incoming Teams bot activities (HTTP POST).
func (h *TeamsHandler) HandleActivity(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var activity teamsActivity
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Only process message activities.
	if activity.Type != "message" {
		w.WriteHeader(http.StatusOK)
		return
	}

	msg := IncomingMessage{
		Platform:  PlatformTeams,
		ChannelID: activity.Conversation.ID,
		UserID:    activity.From.ID,
		UserName:  activity.From.Name,
		Text:      activity.Text,
		ThreadID:  activity.ReplyToID,
		Timestamp: activity.Timestamp,
	}

	resp, err := h.gateway.Process(r.Context(), msg)
	if err != nil {
		http.Error(w, "processing error", http.StatusInternalServerError)
		return
	}

	// Return a Teams activity response.
	teamsResp := map[string]string{
		"type": "message",
		"text": resp.Text,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teamsResp)
}
