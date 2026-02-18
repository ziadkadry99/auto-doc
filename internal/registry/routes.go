package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// RoutesDeps holds the dependencies needed to register repo management routes.
type RoutesDeps struct {
	Store    *Store
	VecStore vectordb.VectorStore
	Tier     config.QualityTier
	OutputDir string
}

// RegisterRoutes wires up the repo management REST API endpoints.
func RegisterRoutes(r chi.Router, deps RoutesDeps) {
	h := &routeHandler{deps: deps}
	r.Route("/api/repos", func(r chi.Router) {
		r.Post("/", h.addRepo)
		r.Get("/", h.listRepos)
		r.Get("/{name}", h.getRepo)
		r.Delete("/{name}", h.removeRepo)
		r.Post("/{name}/sync", h.syncRepo)
	})
}

type routeHandler struct {
	deps RoutesDeps
}

type addRepoRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Path        string `json:"path,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

func (h *routeHandler) addRepo(w http.ResponseWriter, r *http.Request) {
	var req addRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.URL == "" && req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "either url or path is required"})
		return
	}

	ctx := r.Context()

	// Check if repo already exists.
	existing, _ := h.deps.Store.Get(ctx, req.Name)
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("repository %q already registered", req.Name)})
		return
	}

	repo := &Repository{
		Name:        req.Name,
		DisplayName: req.DisplayName,
	}
	if repo.DisplayName == "" {
		repo.DisplayName = req.Name
	}

	if req.URL != "" {
		homeDir, _ := os.UserHomeDir()
		cloneDir := filepath.Join(homeDir, ".autodoc", "repos", req.Name)
		os.MkdirAll(filepath.Dir(cloneDir), 0o755)

		if _, statErr := os.Stat(cloneDir); statErr == nil {
			pullCmd := exec.Command("git", "-C", cloneDir, "pull")
			if err := pullCmd.Run(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("git pull failed: %v", err)})
				return
			}
		} else {
			cloneCmd := exec.Command("git", "clone", req.URL, cloneDir)
			if err := cloneCmd.Run(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("git clone failed: %v", err)})
				return
			}
		}

		repo.SourceType = "git"
		repo.SourceURL = req.URL
		repo.LocalPath = cloneDir
	} else {
		absPath, _ := filepath.Abs(req.Path)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("path does not exist: %s", absPath)})
			return
		}
		repo.SourceType = "local"
		repo.LocalPath = absPath
	}

	if err := h.deps.Store.Add(ctx, repo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("registering repository: %v", err)})
		return
	}

	// Import in background-ish (synchronous for now).
	importer := NewImporter(h.deps.Store, h.deps.VecStore, h.deps.Tier)
	if err := importer.ImportRepo(ctx, repo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("importing repository: %v", err)})
		return
	}

	// Persist vector store.
	vectorDir := filepath.Join(h.deps.OutputDir, "vectordb")
	os.MkdirAll(vectorDir, 0o755)
	h.deps.VecStore.Persist(context.Background(), vectorDir)

	writeJSON(w, http.StatusCreated, repo)
}

func (h *routeHandler) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.deps.Store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("listing repos: %v", err)})
		return
	}
	if repos == nil {
		repos = []Repository{}
	}
	writeJSON(w, http.StatusOK, repos)
}

func (h *routeHandler) getRepo(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	repo, err := h.deps.Store.Get(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("getting repo: %v", err)})
		return
	}
	if repo == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("repository %q not found", name)})
		return
	}

	// Include links.
	links, _ := h.deps.Store.GetLinks(r.Context(), name)
	type repoWithLinks struct {
		*Repository
		Links []ServiceLink `json:"links"`
	}
	writeJSON(w, http.StatusOK, repoWithLinks{Repository: repo, Links: links})
}

func (h *routeHandler) removeRepo(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ctx := r.Context()

	repo, _ := h.deps.Store.Get(ctx, name)
	if repo == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("repository %q not found", name)})
		return
	}

	// Clean up vector store.
	h.deps.VecStore.DeleteByRepoID(ctx, name)
	vectorDir := filepath.Join(h.deps.OutputDir, "vectordb")
	h.deps.VecStore.Persist(context.Background(), vectorDir)

	if err := h.deps.Store.Remove(ctx, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("removing repo: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("repository %q removed", name)})
}

func (h *routeHandler) syncRepo(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ctx := r.Context()

	repo, err := h.deps.Store.Get(ctx, name)
	if err != nil || repo == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("repository %q not found", name)})
		return
	}

	// Git pull if remote.
	if repo.SourceType == "git" {
		pullCmd := exec.Command("git", "-C", repo.LocalPath, "pull")
		if err := pullCmd.Run(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("git pull failed: %v", err)})
			return
		}
	}

	// Re-import.
	importer := NewImporter(h.deps.Store, h.deps.VecStore, h.deps.Tier)
	if err := importer.ImportRepo(ctx, repo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("importing: %v", err)})
		return
	}

	// Persist.
	vectorDir := filepath.Join(h.deps.OutputDir, "vectordb")
	h.deps.VecStore.Persist(context.Background(), vectorDir)

	writeJSON(w, http.StatusOK, repo)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
