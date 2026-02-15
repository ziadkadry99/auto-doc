package backlog

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the knowledge backlog API routes.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Route("/api/backlog", func(r chi.Router) {
		r.Get("/", handleList(store))
		r.Post("/", handleCreate(store))
		r.Get("/stats", handleStats(store))
		r.Get("/top", handleTop(store))
		r.Get("/{id}", handleGetByID(store))
		r.Post("/{id}/answer", handleAnswer(store))
		r.Put("/{id}/status", handleUpdateStatus(store))
	})
}

func handleList(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := ListFilter{}
		if v := r.URL.Query().Get("repo_id"); v != "" {
			filter.RepoID = v
		}
		if v := r.URL.Query().Get("status"); v != "" {
			filter.Status = Status(v)
		}
		if v := r.URL.Query().Get("category"); v != "" {
			filter.Category = Category(v)
		}
		if v := r.URL.Query().Get("min_priority"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.MinPriority = n
			}
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Offset = n
			}
		}

		questions, err := store.List(r.Context(), filter)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if questions == nil {
			questions = []Question{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(questions)
	}
}

func handleCreate(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q Question
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if q.Question == "" {
			http.Error(w, `{"error":"question is required"}`, http.StatusBadRequest)
			return
		}

		created, err := store.Create(r.Context(), q)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func handleGetByID(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		q, err := store.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if q == nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(q)
	}
}

type answerRequest struct {
	Answer     string `json:"answer"`
	AnsweredBy string `json:"answered_by"`
}

func handleAnswer(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req answerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Answer == "" {
			http.Error(w, `{"error":"answer is required"}`, http.StatusBadRequest)
			return
		}
		if req.AnsweredBy == "" {
			req.AnsweredBy = "anonymous"
		}

		if err := store.Answer(r.Context(), id, req.Answer, req.AnsweredBy); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "answered"})
	}
}

type statusRequest struct {
	Status Status `json:"status"`
}

func handleUpdateStatus(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req statusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if err := store.UpdateStatus(r.Context(), id, req.Status); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": string(req.Status)})
	}
}

func handleStats(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := store.GetOpenCount(r.Context())
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"open_count": count})
	}
}

func handleTop(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 10
		if v := r.URL.Query().Get("n"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				n = parsed
			}
		}

		questions, err := store.GetTopPriority(r.Context(), n)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if questions == nil {
			questions = []Question{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(questions)
	}
}
