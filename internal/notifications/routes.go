package notifications

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts notification endpoints under /api/notifications on the given router.
func RegisterRoutes(r chi.Router, store *Store, dispatcher *Dispatcher) {
	r.Route("/api/notifications", func(r chi.Router) {
		r.Get("/", handleList(store))
		r.Get("/pending", handlePending(store))
		r.Get("/digest/{teamID}", handleDigest(dispatcher))
		r.Get("/preferences/{teamID}", handleGetPreferences(store))
		r.Put("/preferences", handleSetPreference(store))
		r.Get("/{id}", handleGetByID(store))
		r.Post("/{id}/deliver", handleMarkDelivered(store))
	})
}

func handleList(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filter := ListFilter{}

		if v := q.Get("type"); v != "" {
			filter.Type = NotificationType(v)
		}
		if v := q.Get("severity"); v != "" {
			filter.Severity = Severity(v)
		}
		if v := q.Get("delivered"); v != "" {
			b, err := strconv.ParseBool(v)
			if err == nil {
				filter.Delivered = &b
			}
		}
		if v := q.Get("since"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.Since = t
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

		notifications, err := store.List(r.Context(), filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, notifications)
	}
}

func handleGetByID(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		n, err := store.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, n)
	}
}

func handleMarkDelivered(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		if err := store.MarkDelivered(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "delivered"})
	}
}

func handlePending(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notifications, err := store.GetPending(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, notifications)
	}
}

func handleGetPreferences(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "teamID")

		prefs, err := store.GetPreferences(r.Context(), teamID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, prefs)
	}
}

func handleSetPreference(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pref Preference
		if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if pref.TeamID == "" || pref.Channel == "" {
			http.Error(w, "team_id and channel are required", http.StatusBadRequest)
			return
		}

		if err := store.SetPreference(r.Context(), pref); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, pref)
	}
}

func handleDigest(dispatcher *Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "teamID")

		since := time.Now().UTC().Add(-24 * time.Hour) // default: last 24 hours
		if v := r.URL.Query().Get("since"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				since = t
			}
		}

		digest, err := dispatcher.GenerateDigest(r.Context(), teamID, since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, digest)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
