package confidence

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the confidence API endpoints on the given router.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Get("/api/confidence", listHandler(store))
	r.Get("/api/confidence/stats", statsHandler(store))
	r.Get("/api/confidence/{entityType}/{entityID}", getHandler(store))
	r.Put("/api/confidence", setHandler(store))
}

func listHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := ListFilter{}
		if v := r.URL.Query().Get("entity_type"); v != "" {
			filter.EntityType = EntityType(v)
		}
		if v := r.URL.Query().Get("confidence"); v != "" {
			filter.Confidence = Level(v)
		}
		if v := r.URL.Query().Get("stale_only"); v == "true" || v == "1" {
			filter.StaleOnly = true
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				filter.Limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				filter.Offset = n
			}
		}

		results, err := store.List(r.Context(), filter)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if results == nil {
			results = []Metadata{}
		}
		writeJSON(w, http.StatusOK, results)
	}
}

func getHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityType := EntityType(chi.URLParam(r, "entityType"))
		entityID := chi.URLParam(r, "entityID")

		meta, err := store.Get(r.Context(), entityType, entityID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if meta == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, meta)
	}
}

func setHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var meta Metadata
		if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()})
			return
		}
		if meta.EntityType == "" || meta.EntityID == "" || meta.Confidence == "" || meta.Source == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity_type, entity_id, confidence, and source are required"})
			return
		}

		if err := store.Set(r.Context(), meta); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func statsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := store.Stats(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
