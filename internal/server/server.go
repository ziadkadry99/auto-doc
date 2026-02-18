package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ziadkadry99/auto-doc/internal/db"
	"github.com/ziadkadry99/auto-doc/internal/embeddings"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// Config holds server configuration.
type Config struct {
	Port     int
	DataDir  string // directory for SQLite DB and data files
	DocsDir  string // directory containing generated docs
	AllowAll bool   // allow all CORS origins (dev mode)
}

// Server is the Phase 4 central documentation server.
type Server struct {
	cfg         Config
	db          *db.DB
	store       vectordb.VectorStore
	embedder    embeddings.Embedder
	llmProvider llm.Provider
	llmModel    string
	router      chi.Router
	httpServer  *http.Server
}

// New creates a new Phase 4 server with all dependencies.
func New(cfg Config, database *db.DB, store vectordb.VectorStore, embedder embeddings.Embedder, llmProvider llm.Provider, llmModel string) *Server {
	s := &Server{
		cfg:         cfg,
		db:          database,
		store:       store,
		embedder:    embedder,
		llmProvider: llmProvider,
		llmModel:    llmModel,
	}

	s.router = s.buildRouter()
	return s
}

// buildRouter creates and configures the chi router with all routes.
func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	corsOpts := cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	if s.cfg.AllowAll {
		corsOpts.AllowedOrigins = []string{"*"}
	}
	r.Use(cors.Handler(corsOpts))

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API routes are registered by feature packages via RegisterRoutes.
	// The server exposes the router and DB for feature packages to use.

	return r
}

// Router returns the chi router for registering additional routes.
func (s *Server) Router() chi.Router { return s.router }

// DB returns the database connection.
func (s *Server) Database() *db.DB { return s.db }

// Store returns the vector store.
func (s *Server) Store() vectordb.VectorStore { return s.store }

// Embedder returns the embedder.
func (s *Server) Embedder() embeddings.Embedder { return s.embedder }

// LLMProvider returns the LLM provider.
func (s *Server) LLMProvider() llm.Provider { return s.llmProvider }

// LLMModel returns the configured LLM model name.
func (s *Server) LLMModel() string { return s.llmModel }

// Config returns the server configuration.
func (s *Server) ServerConfig() Config { return s.cfg }

// Start begins listening on the configured port.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("autodoc server listening on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
