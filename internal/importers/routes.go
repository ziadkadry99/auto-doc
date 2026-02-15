package importers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the import sources API routes.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Route("/api/imports", func(r chi.Router) {
		r.Get("/", handleList(store))
		r.Post("/", handleCreate(store))
		r.Get("/{id}", handleGetByID(store))
		r.Delete("/{id}", handleDelete(store))
	})
}

func handleList(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sources, err := store.List(r.Context())
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if sources == nil {
			sources = []ImportSource{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sources)
	}
}

func handleCreate(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var src ImportSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if src.Name == "" || src.Type == "" {
			http.Error(w, `{"error":"name and type are required"}`, http.StatusBadRequest)
			return
		}

		created, err := store.Create(r.Context(), src)
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
		src, err := store.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if src == nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(src)
	}
}

func handleDelete(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := store.Delete(r.Context(), id); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
