package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ziadkadry99/auto-doc/internal/audit"
	"github.com/ziadkadry99/auto-doc/internal/backlog"
	"github.com/ziadkadry99/auto-doc/internal/bots"
	"github.com/ziadkadry99/auto-doc/internal/confidence"
	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
	"github.com/ziadkadry99/auto-doc/internal/dashboard"
	"github.com/ziadkadry99/auto-doc/internal/db"
	"github.com/ziadkadry99/auto-doc/internal/flows"
	"github.com/ziadkadry99/auto-doc/internal/importers"
	"github.com/ziadkadry99/auto-doc/internal/notifications"
	"github.com/ziadkadry99/auto-doc/internal/orgstructure"
	"github.com/ziadkadry99/auto-doc/internal/server"
	"github.com/ziadkadry99/auto-doc/internal/vectordb"
)

var serverPort int

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Phase 4 central documentation server",
	Long:  `Starts the autodoc central documentation server with REST API, chat dashboard, and multi-repo support.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Create embedder.
		embedder, err := createEmbedderFromConfig(cfg)
		if err != nil {
			return fmt.Errorf("creating embedder: %w", err)
		}

		// Create and load vector store.
		store, err := vectordb.NewChromemStore(embedder)
		if err != nil {
			return fmt.Errorf("creating vector store: %w", err)
		}

		vectorDir := filepath.Join(cfg.OutputDir, "vectordb")
		if err := store.Load(context.Background(), vectorDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load vector store from %s: %v\n", vectorDir, err)
		}

		// Create LLM provider.
		llmProvider, err := createLLMProviderFromConfig(cfg)
		if err != nil {
			return fmt.Errorf("creating LLM provider: %w", err)
		}

		// Open database.
		dbPath := filepath.Join(cfg.OutputDir, "autodoc.db")
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer database.Close()

		// Create and start server.
		srv := server.New(server.Config{
			Port:     serverPort,
			DataDir:  cfg.OutputDir,
			DocsDir:  cfg.OutputDir,
			AllowAll: true,
		}, database, store, embedder, llmProvider, cfg.Model)

		// Register all feature routes.
		registerAllRoutes(srv, database, llmProvider, cfg.Model, store)

		// Graceful shutdown.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			<-ctx.Done()
			fmt.Fprintln(os.Stderr, "\nShutting down server...")
			srv.Shutdown(context.Background())
		}()

		fmt.Fprintf(os.Stderr, "autodoc server v%s starting on port %d\n", Version, serverPort)
		fmt.Fprintf(os.Stderr, "  Database: %s\n", dbPath)
		fmt.Fprintf(os.Stderr, "  Docs: %s\n", cfg.OutputDir)
		fmt.Fprintf(os.Stderr, "  Documents indexed: %d\n", store.Count())

		return srv.Start()
	},
}

// registerAllRoutes wires up all Phase 4 feature routes.
func registerAllRoutes(srv *server.Server, database *db.DB, llmProvider interface{}, model string, store vectordb.VectorStore) {
	r := srv.Router()

	// Audit Trail
	auditStore := audit.NewStore(database)
	audit.RegisterRoutes(r, auditStore)

	// Confidence Badges
	confStore := confidence.NewStore(database)
	confidence.RegisterRoutes(r, confStore)

	// Org Structure
	orgStore := orgstructure.NewStore(database)
	orgstructure.RegisterRoutes(r, orgStore)

	// Flows
	flowStore := flows.NewStore(database)
	flows.RegisterRoutes(r, flowStore)

	// Notifications
	notifStore := notifications.NewStore(database)
	notifDispatcher := notifications.NewDispatcher(notifStore)
	notifications.RegisterRoutes(r, notifStore, notifDispatcher)

	// Knowledge Backlog
	backlogStore := backlog.NewStore(database)
	backlog.RegisterRoutes(r, backlogStore)

	// Context Engine
	ctxStore := contextengine.NewStore(database)
	ctxEngine := contextengine.NewEngine(ctxStore, srv.LLMProvider(), srv.LLMModel())
	contextengine.RegisterRoutes(r, ctxEngine)

	// Importers
	importStore := importers.NewStore(database)
	importers.RegisterRoutes(r, importStore)

	// Dashboard (chat-first UI)
	dash := dashboard.New(ctxEngine, store, srv.LLMProvider(), srv.LLMModel(), backlogStore)
	dash.RegisterRoutes(r)

	// Bots (Slack & Teams)
	botProcessor := bots.NewProcessor(ctxEngine, backlogStore)
	botGateway := bots.NewGateway(botProcessor)
	slackHandler := bots.NewSlackHandler(botGateway, "")
	teamsHandler := bots.NewTeamsHandler(botGateway)
	bots.RegisterRoutes(r, slackHandler, teamsHandler)

	_ = confStore
	_ = orgStore
	_ = flowStore
	_ = notifDispatcher
}

func init() {
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "Port to listen on")
	rootCmd.AddCommand(serverCmd)
}
