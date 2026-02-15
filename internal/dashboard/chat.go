package dashboard

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// chatRequest is the incoming WebSocket message format.
type chatRequest struct {
	Type      string `json:"type"`       // "message" or "ask"
	SessionID string `json:"session_id"` // empty for new sessions
	Content   string `json:"content"`
}

// chatResponse is the outgoing WebSocket message format.
type chatResponse struct {
	Type           string `json:"type"`            // "response" or "error"
	SessionID      string `json:"session_id"`
	Content        string `json:"content"`
	FactsExtracted int    `json:"facts_extracted,omitempty"`
}

func (d *Dashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("dashboard: websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("dashboard: websocket read: %v", err)
			}
			return
		}

		var req chatRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			d.sendError(conn, "", "invalid message format")
			continue
		}

		if req.Content == "" {
			d.sendError(conn, req.SessionID, "content is required")
			continue
		}

		switch req.Type {
		case "message":
			d.handleChatMessage(conn, r, req)
		case "ask":
			d.handleAskMessage(conn, r, req)
		default:
			d.sendError(conn, req.SessionID, "unknown message type: "+req.Type)
		}
	}
}

func (d *Dashboard) handleChatMessage(conn *websocket.Conn, r *http.Request, req chatRequest) {
	if d.llmProvider == nil {
		d.sendError(conn, req.SessionID, "LLM provider not configured")
		return
	}

	ctx := r.Context()
	sessionID := req.SessionID

	// Create a new session if needed.
	if sessionID == "" {
		sess, err := d.engine.Store().CreateSession(ctx, "dashboard")
		if err != nil {
			d.sendError(conn, "", "failed to create session: "+err.Error())
			return
		}
		sessionID = sess.ID
	}

	update, err := d.engine.ProcessInput(ctx, sessionID, "dashboard", req.Content)
	if err != nil {
		d.sendError(conn, sessionID, "processing failed: "+err.Error())
		return
	}

	resp := chatResponse{
		Type:           "response",
		SessionID:      sessionID,
		Content:        update.Summary,
		FactsExtracted: len(update.Facts),
	}
	d.sendResponse(conn, resp)
}

func (d *Dashboard) handleAskMessage(conn *websocket.Conn, r *http.Request, req chatRequest) {
	if d.llmProvider == nil {
		d.sendError(conn, req.SessionID, "LLM provider not configured")
		return
	}

	answer, err := d.engine.AskQuestion(r.Context(), req.Content)
	if err != nil {
		d.sendError(conn, req.SessionID, "question failed: "+err.Error())
		return
	}

	resp := chatResponse{
		Type:      "response",
		SessionID: req.SessionID,
		Content:   answer,
	}
	d.sendResponse(conn, resp)
}

func (d *Dashboard) sendResponse(conn *websocket.Conn, resp chatResponse) {
	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("dashboard: websocket write: %v", err)
	}
}

func (d *Dashboard) sendError(conn *websocket.Conn, sessionID, message string) {
	resp := chatResponse{
		Type:      "error",
		SessionID: sessionID,
		Content:   message,
	}
	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("dashboard: websocket write error: %v", err)
	}
}
