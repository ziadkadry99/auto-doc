package flows

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts flow endpoints on the given router.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Get("/api/flows", listFlowsHandler(store))
	r.Get("/api/flows/{id}", getFlowHandler(store))
	r.Post("/api/flows", createFlowHandler(store))
	r.Put("/api/flows/{id}", updateFlowHandler(store))
}

func listFlowsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		var (
			result []Flow
			err    error
		)
		if q != "" {
			result, err = store.SearchFlows(r.Context(), q)
		} else {
			result, err = store.ListFlows(r.Context())
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if result == nil {
			result = []Flow{}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func getFlowHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		flow, err := store.GetFlow(r.Context(), id)
		if err != nil {
			http.Error(w, "flow not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, flow)
	}
}

func createFlowHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var f Flow
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if f.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if err := store.CreateFlow(r.Context(), &f); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, f)
	}
}

func updateFlowHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var f Flow
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		f.ID = id
		if err := store.UpdateFlow(r.Context(), &f); err != nil {
			http.Error(w, "flow not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
