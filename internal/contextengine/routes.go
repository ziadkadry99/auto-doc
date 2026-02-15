package contextengine

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the context engine API routes.
func RegisterRoutes(r chi.Router, engine *Engine) {
	r.Route("/api/context", func(r chi.Router) {
		r.Post("/process", handleProcess(engine))
		r.Post("/ask", handleAsk(engine))
		r.Post("/correct", handleCorrect(engine))
		r.Get("/facts", handleListFacts(engine))
		r.Get("/facts/search", handleSearchFacts(engine))
		r.Get("/facts/{id}", handleGetFact(engine))
		r.Get("/facts/history", handleFactHistory(engine))
		r.Post("/sessions", handleCreateSession(engine))
		r.Get("/sessions/{id}/messages", handleGetMessages(engine))
	})
}

type processRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Input     string `json:"input"`
}

func handleProcess(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req processRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Input == "" {
			http.Error(w, `{"error":"input is required"}`, http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			req.UserID = "anonymous"
		}

		update, err := engine.ProcessInput(r.Context(), req.SessionID, req.UserID, req.Input)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(update)
	}
}

type askRequest struct {
	Question string `json:"question"`
}

func handleAsk(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req askRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Question == "" {
			http.Error(w, `{"error":"question is required"}`, http.StatusBadRequest)
			return
		}

		answer, err := engine.AskQuestion(r.Context(), req.Question)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"answer": answer})
	}
}

type correctRequest struct {
	UserID     string     `json:"user_id"`
	Correction Correction `json:"correction"`
}

func handleCorrect(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req correctRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			req.UserID = "anonymous"
		}

		update, err := engine.ProcessCorrection(r.Context(), req.UserID, req.Correction)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(update)
	}
}

func handleListFacts(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoID := r.URL.Query().Get("repo_id")
		scope := r.URL.Query().Get("scope")
		scopeID := r.URL.Query().Get("scope_id")

		facts, err := engine.store.GetCurrentFacts(r.Context(), repoID, scope, scopeID)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if facts == nil {
			facts = []Fact{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(facts)
	}
}

func handleSearchFacts(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, `{"error":"q parameter is required"}`, http.StatusBadRequest)
			return
		}
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		facts, err := engine.store.SearchFacts(r.Context(), query, limit)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if facts == nil {
			facts = []Fact{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(facts)
	}
}

func handleGetFact(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		fact, err := engine.store.GetFact(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if fact == nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fact)
	}
}

func handleFactHistory(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoID := r.URL.Query().Get("repo_id")
		scope := r.URL.Query().Get("scope")
		scopeID := r.URL.Query().Get("scope_id")
		key := r.URL.Query().Get("key")

		if scope == "" || key == "" {
			http.Error(w, `{"error":"scope and key are required"}`, http.StatusBadRequest)
			return
		}

		facts, err := engine.store.GetFactHistory(r.Context(), repoID, scope, scopeID, key)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if facts == nil {
			facts = []Fact{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(facts)
	}
}

type createSessionRequest struct {
	UserID string `json:"user_id"`
}

func handleCreateSession(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.UserID = "anonymous"
		}
		if req.UserID == "" {
			req.UserID = "anonymous"
		}

		sess, err := engine.store.CreateSession(r.Context(), req.UserID)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sess)
	}
}

func handleGetMessages(engine *Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "id")
		messages, err := engine.store.GetMessages(r.Context(), sessionID)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if messages == nil {
			messages = []ConversationMessage{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}
}
