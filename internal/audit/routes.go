package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts audit endpoints under /api/audit on the given router.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Route("/api/audit", func(r chi.Router) {
		r.Get("/", handleQuery(store))
		r.Get("/{id}", handleGetByID(store))
	})
}

func handleQuery(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filter := QueryFilter{
			ActorID:         q.Get("actor"),
			ScopeID:         q.Get("scope_id"),
			AffectedService: q.Get("service"),
		}

		if v := q.Get("scope"); v != "" {
			filter.Scope = Scope(v)
		}
		if v := q.Get("action"); v != "" {
			filter.Action = Action(v)
		}
		if v := q.Get("since"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.Since = &t
			}
		}
		if v := q.Get("until"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.Until = &t
			}
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Limit = n
			}
		}
		if v := q.Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Offset = n
			}
		}

		entries, err := store.Query(r.Context(), filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, entries)
	}
}

func handleGetByID(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		entry, err := store.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, entry)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
