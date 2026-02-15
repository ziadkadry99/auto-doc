package orgstructure

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts team and ownership endpoints on the given router.
func RegisterRoutes(r chi.Router, store *Store) {
	r.Get("/api/teams", listTeamsHandler(store))
	r.Get("/api/teams/{id}", getTeamHandler(store))
	r.Post("/api/teams", createTeamHandler(store))
	r.Put("/api/teams/{id}", updateTeamHandler(store))
	r.Get("/api/teams/{id}/services", listTeamServicesHandler(store))
	r.Get("/api/ownership/{repoID}", getOwnershipHandler(store))
}

func listTeamsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teams, err := store.ListTeams(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if teams == nil {
			teams = []Team{}
		}
		writeJSON(w, http.StatusOK, teams)
	}
}

func getTeamHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		team, err := store.GetTeam(r.Context(), id)
		if err != nil {
			http.Error(w, "team not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, team)
	}
}

func createTeamHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var t Team
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if t.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if err := store.CreateTeam(r.Context(), &t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, t)
	}
}

func updateTeamHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var t Team
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		t.ID = id
		if err := store.UpdateTeam(r.Context(), &t); err != nil {
			http.Error(w, "team not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func listTeamServicesHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		ownerships, err := store.ListOwnerships(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if ownerships == nil {
			ownerships = []ServiceOwnership{}
		}
		writeJSON(w, http.StatusOK, ownerships)
	}
}

func getOwnershipHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoID := chi.URLParam(r, "repoID")
		ownerships, err := store.GetOwnership(r.Context(), repoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if ownerships == nil {
			ownerships = []ServiceOwnership{}
		}
		writeJSON(w, http.StatusOK, ownerships)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
