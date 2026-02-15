package dashboard

import (
	"github.com/go-chi/chi/v5"
	"github.com/ziadkadry99/auto-doc/internal/backlog"
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/llm"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

// Dashboard provides the chat-first dashboard and AI search interface.
type Dashboard struct {
	router       chi.Router
	engine       *contextengine.Engine
	vectorStore  vectordb.VectorStore
	llmProvider  llm.Provider
	llmModel     string
	backlogStore *backlog.Store
}

// New creates a new Dashboard.
func New(engine *contextengine.Engine, store vectordb.VectorStore, llmProvider llm.Provider, llmModel string, backlogStore *backlog.Store) *Dashboard {
	return &Dashboard{
		router:       chi.NewRouter(),
		engine:       engine,
		vectorStore:  store,
		llmProvider:  llmProvider,
		llmModel:     llmModel,
		backlogStore: backlogStore,
	}
}

// RegisterRoutes mounts all dashboard routes onto the given router.
func (d *Dashboard) RegisterRoutes(r chi.Router) {
	r.Get("/", d.ServeIndex)
	r.Get("/api/dashboard/stats", d.handleStats)
	r.Get("/api/dashboard/recent", d.handleRecent)
	r.Get("/ws/chat", d.handleWebSocket)
}
