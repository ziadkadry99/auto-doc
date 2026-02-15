<p align="center">
  <img src="assets/logo.png" width="120" alt="autodoc logo"/>
</p>

<h1 align="center">autodoc</h1>

<p align="center">
  <strong>AI-powered codebase documentation and semantic search.</strong><br/>
  Point it at any repo. Get a full documentation site with architecture diagrams, interactive component maps, and natural language search — in minutes.
</p>

<p align="center">
  <a href="#installation">Installation</a> &bull;
  <a href="#quick-start">Quick Start</a> &bull;
  <a href="#features">Features</a> &bull;
  <a href="#commands">Commands</a> &bull;
  <a href="#configuration">Configuration</a> &bull;
  <a href="#github-pages">GitHub Pages</a> &bull;
  <a href="#mcp-integration">MCP Integration</a>
</p>

---

## What It Does

`autodoc` reads your entire codebase with AI, then generates:

- **Markdown documentation** for every file — summaries, key functions, classes, dependencies
- **An enhanced home page** with a project overview, feature groupings, and architecture diagrams
- **A static documentation site** with full-text search, Mermaid diagrams, and dark/light themes
- **An interactive component map** — a D3.js force-directed graph showing how your files and packages connect
- **A semantic vector database** — search your codebase in natural language from the CLI or through AI agents
- **An MCP server** — plug directly into Claude Code, Cursor, or any MCP-compatible AI agent for instant codebase understanding

It works with **any language**, **any size** codebase.

## Features

### Multi-Provider LLM Support

Use the AI provider you prefer — or run fully local with Ollama:

| Provider | Models | API Key |
|----------|--------|---------|
| **Anthropic** | Claude Sonnet, Haiku | `ANTHROPIC_API_KEY` |
| **OpenAI** | GPT-4o, GPT-4o-mini | `OPENAI_API_KEY` |
| **Google** | Gemini 2.0 Flash/Pro | `GOOGLE_API_KEY` |
| **Ollama** | Any local model | None (local) |

### Quality Tiers

Choose the depth-vs-cost tradeoff that fits:

| Tier | What You Get | Best For |
|------|-------------|----------|
| **Lite** | File summaries, basic index | Large codebases, quick overviews |
| **Normal** | + function/class analysis, architecture overview | Day-to-day use |
| **Max** | + dependency graphs, detailed analysis | Deep documentation |

### Static Documentation Site

Generate a self-contained HTML site with:

- Responsive layout with dark/light theme toggle
- Full-text search across all documentation
- AI-powered search answers (synthesized by your LLM)
- Mermaid architecture and dependency diagrams
- Interactive D3.js component map with feature clustering
- Per-file documentation pages with function/class tables

### Incremental Updates

After the initial generation, `autodoc update` detects changes via `git diff` and only re-processes modified files — saving time and API costs.

### Business Context

Provide optional project context (what the project does, who it's for, key architectural decisions) to produce more accurate, domain-aware documentation:

```bash
autodoc generate --interactive   # prompted wizard
autodoc generate --context-file context.json  # from file
```

### MCP Server for AI Agents

Expose your indexed codebase to AI agents via the [Model Context Protocol](https://modelcontextprotocol.io):

```json
{
  "mcpServers": {
    "autodoc": {
      "command": "autodoc",
      "args": ["serve"]
    }
  }
}
```

Tools exposed: `search_codebase`, `get_file_docs`, `get_architecture`, `get_diagram`.

## Installation

### From Source

```bash
go install github.com/ziadkadry99/auto-doc@latest
```

### From Release Binaries

Download from the [Releases](https://github.com/ziadkadry99/auto-doc/releases) page, or use the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/ziadkadry99/auto-doc/main/scripts/install.sh | sh
```

### Build from Source

```bash
git clone https://github.com/ziadkadry99/auto-doc.git
cd auto-doc
make build
```

## Quick Start

```bash
# 1. Initialize — interactive wizard picks your provider, model, and quality tier
autodoc init

# 2. Generate documentation
autodoc generate

# 3. Browse it
autodoc site --serve
```

That's it. Open `http://localhost:8080` and explore your codebase.

## Commands

| Command | Description |
|---------|-------------|
| `autodoc init` | Interactive setup wizard — creates `.autodoc.yml` |
| `autodoc generate` | Full documentation generation + vector index |
| `autodoc update` | Incremental update — only re-processes changed files |
| `autodoc site` | Generate static HTML documentation site |
| `autodoc site --serve` | Generate and serve locally with live search |
| `autodoc query "..."` | Semantic search from the command line |
| `autodoc serve` | Start MCP server for AI agent integration |
| `autodoc cost` | Estimate API costs before generating |
| `autodoc version` | Print version |

### Key Flags

```bash
autodoc generate --interactive       # Collect business context via prompts
autodoc generate --context-file ctx.json  # Load business context from file
autodoc generate --dry-run           # Estimate costs without API calls
autodoc generate --concurrency 8     # Control parallel LLM calls

autodoc update --force               # Re-process all files (skip git diff)

autodoc site --serve --port 9090     # Serve on custom port
autodoc site --serve --open          # Auto-open browser

autodoc query "how does auth work" --json --limit 5
```

## Configuration

`autodoc init` generates `.autodoc.yml`:

```yaml
provider: anthropic          # anthropic, openai, google, ollama
model: claude-sonnet-4-5-20250929
embedding_provider: openai   # openai, google, or ollama
embedding_model: text-embedding-3-small
quality: normal              # lite, normal, max
output_dir: .autodoc
logo: assets/logo.png        # optional — logo displayed in the docs site sidebar
max_concurrency: 4

include:
  - "**/*"
exclude:
  - "node_modules/**"
  - "vendor/**"
  - ".git/**"
  - "dist/**"
```

### Logo

To display a logo in the generated documentation site sidebar, set the `logo` field in `.autodoc.yml` to the path of an image file (relative to the project root):

```yaml
logo: assets/logo.png
```

The logo appears above the project title in the sidebar navigation. Supported formats: PNG, JPG, SVG. The image is automatically copied into the generated site output.

### Environment Variables

| Variable | Required For |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic provider |
| `OPENAI_API_KEY` | OpenAI provider / OpenAI embeddings |
| `GOOGLE_API_KEY` | Google provider |
| `OLLAMA_HOST` | Custom Ollama endpoint (default: `http://localhost:11434`) |

## GitHub Pages

autodoc includes a GitHub Actions workflow to automatically generate and deploy your documentation to GitHub Pages on every push.

### Setup

1. **Enable GitHub Pages** in your repo settings: Settings > Pages > Source > **GitHub Actions**

2. **Add repository secret** (Settings > Secrets and variables > Actions):
   - `GOOGLE_API_KEY` — your Google AI API key (used for both generation and embeddings)

3. **Push to `main`** — the workflow runs automatically, or trigger it manually from the Actions tab.

The workflow builds autodoc from source, generates documentation, creates the static site, and deploys it. The CI config uses `gemini-2.0-flash` with `normal` quality for faster/cheaper builds.

To customize the CI generation settings, edit `.github/workflows/pages.yml` and modify the inline `.autodoc.ci.yml` config.

## MCP Integration

autodoc exposes an MCP server for AI agents to understand your codebase instantly.

### Claude Code

Add to your project's `.claude/settings.json`:

```json
{
  "mcpServers": {
    "autodoc": {
      "command": "autodoc",
      "args": ["serve"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `search_codebase` | Semantic search across all indexed docs |
| `get_file_docs` | Full AI-generated docs for a specific file |
| `get_architecture` | High-level architecture overview |
| `get_diagram` | Mermaid diagrams (architecture, dependency, sequence) |

## Project Structure

```
cmd/                    CLI commands (Cobra)
internal/
  config/               Configuration, interactive wizard
  walker/               Codebase traversal, .gitignore support
  llm/                  Multi-provider LLM abstraction
  embeddings/           Embedding generation (OpenAI, Ollama)
  vectordb/             Vector store (chromem-go)
  indexer/              Core pipeline — analysis, chunking, batching, state
  docs/                 Markdown generation, features, interactive map
  diagrams/             Mermaid diagram generation
  site/                 Static site generator + dev server
  mcp/                  MCP server implementation
  context/              Business context collection + persistence
  progress/             Terminal progress reporting
scripts/
  install.sh            Curl-pipe installer
```

## How It Works

1. **Walk** — Traverses the codebase respecting `.gitignore`, include/exclude patterns, and language detection
2. **Analyze** — Sends each file to the LLM with structured prompts. Extracts summaries, purposes, functions, classes, dependencies
3. **Chunk** — Splits analysis into semantic chunks (file-level, function-level, class-level)
4. **Embed** — Generates vector embeddings for each chunk
5. **Store** — Persists embeddings in a local vector database (chromem-go, pure Go, no CGO)
6. **Document** — Renders markdown docs, enhanced index with features, architecture diagrams, interactive map
7. **Serve** — Generates static HTML site or exposes MCP server for AI agents

## Development

```bash
make build      # Build binary
make test       # Run tests with race detector
make lint       # Run golangci-lint
make tidy       # go mod tidy
make install    # go install with version ldflags
make dist       # GoReleaser snapshot build
make release    # GoReleaser full release
```

## License

MIT

---

<p align="center">
  Built with Go. Powered by AI.<br/>
  <sub>Works with Anthropic, OpenAI, Google Gemini, and Ollama.</sub>
</p>
