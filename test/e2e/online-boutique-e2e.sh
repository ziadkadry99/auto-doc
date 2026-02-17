#!/usr/bin/env bash
# online-boutique-e2e.sh — End-to-end integration test for autodoc
#
# Clones Google's Online Boutique (11 microservices, 5 languages),
# generates full documentation with autodoc, then evaluates retrieval
# and synthesis quality across 10 graded questions.
#
# Prerequisites:
#   - GOOGLE_API_KEY (or GEMINI_API_KEY) set
#   - go, git, curl on PATH
#
# Usage:
#   ./test/e2e/online-boutique-e2e.sh [--skip-generate] [--skip-cost-prompt]

set -euo pipefail

# Source .env if present (for API keys).
if [[ -f "$( cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd )/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$( cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd )/.env"
  set +a
fi

# ─────────────────────────────────────────────────────────────────────
# Constants
# ─────────────────────────────────────────────────────────────────────

REPO_URL="https://github.com/GoogleCloudPlatform/microservices-demo.git"
REPO_DIR=""        # set in main after mktemp
AUTODOC_BIN=""     # set after build
SITE_PID=""        # set when site server starts
SITE_PORT=9999
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_FILE="$SCRIPT_DIR/report-${TIMESTAMP}.md"
RESULTS_DIR=""     # set in main after mktemp
QUERY_TIMEOUT=120  # seconds per query
GENERATE_TIMEOUT=1800  # 30 min for full generation

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'  # No Color

# Detect python binary (python3 or python, whichever is available).
PYTHON_BIN=""
if command -v python3 &>/dev/null; then
  PYTHON_BIN="python3"
elif command -v python &>/dev/null; then
  PYTHON_BIN="python"
fi

# ─────────────────────────────────────────────────────────────────────
# CLI flags
# ─────────────────────────────────────────────────────────────────────

SKIP_GENERATE=false
SKIP_COST_PROMPT=false

for arg in "$@"; do
  case "$arg" in
    --skip-generate)   SKIP_GENERATE=true ;;
    --skip-cost-prompt) SKIP_COST_PROMPT=true ;;
    --help|-h)
      echo "Usage: $0 [--skip-generate] [--skip-cost-prompt]"
      echo ""
      echo "  --skip-generate     Skip doc generation (reuse existing .autodoc dir)"
      echo "  --skip-cost-prompt  Skip the cost estimate confirmation prompt"
      exit 0
      ;;
    *) echo "Unknown flag: $arg"; exit 1 ;;
  esac
done

# ─────────────────────────────────────────────────────────────────────
# The 10 questions + ground truth
# ─────────────────────────────────────────────────────────────────────

declare -a QUESTIONS
declare -a DIFFICULTIES
declare -a GROUND_TRUTHS
declare -a CATEGORIES

QUESTIONS[1]="What programming languages are used in this project?"
DIFFICULTIES[1]="1/5"
CATEGORIES[1]="Basic Facts"
GROUND_TRUTHS[1]="Go, Node.js, Python, C#, Java, Protocol Buffers. Must name 6 languages and associate them with services."

QUESTIONS[2]="What port does the frontend service listen on and what HTTP routes does it expose?"
DIFFICULTIES[2]="2/5"
CATEGORIES[2]="Basic Facts"
GROUND_TRUTHS[2]="Port 8080. Routes include /, /product/{id}, /cart, /cart/checkout, /setCurrency, /_healthz."

QUESTIONS[3]="How do the microservices communicate with each other?"
DIFFICULTIES[3]="2/5"
CATEGORIES[3]="Architecture"
GROUND_TRUTHS[3]="gRPC over Protocol Buffers defined in protos/demo.proto. Frontend translates HTTP to gRPC. Service discovery via env vars."

QUESTIONS[4]="What is the role of Redis in this architecture and which service uses it?"
DIFFICULTIES[4]="2/5"
CATEGORIES[4]="Architecture"
GROUND_TRUTHS[4]="Redis is used by cartservice (C#) for shopping cart persistence, keyed by user ID."

QUESTIONS[5]="Trace the complete flow when a user places an order. Which services are involved and in what sequence?"
DIFFICULTIES[5]="4/5"
CATEGORIES[5]="Cross-Service Tracing"
GROUND_TRUTHS[5]="Frontend -> CheckoutService orchestrates: Cart -> ProductCatalog -> Currency -> Shipping -> Payment -> Cart.EmptyCart -> Email. 7-8 services total."

QUESTIONS[6]="What happens if the currency service goes down? Which services are directly and indirectly affected?"
DIFFICULTIES[6]="4/5"
CATEGORIES[6]="Blast Radius"
GROUND_TRUTHS[6]="Directly: Frontend (price display), CheckoutService (price conversion). Indirectly: all downstream checkout services. Currency is critical in both browsing and purchasing paths."

QUESTIONS[7]="How does the recommendation service decide which products to suggest?"
DIFFICULTIES[7]="3/5"
CATEGORIES[7]="Implementation Detail"
GROUND_TRUTHS[7]="Random selection via random.sample(). Fetches all products from ProductCatalogService, filters out current products, picks up to 5 randomly. No ML — it is a mock."

QUESTIONS[8]="What credit card validation does the payment service perform, and what card types are accepted?"
DIFFICULTIES[8]="3/5"
CATEGORIES[8]="Implementation Detail"
GROUND_TRUTHS[8]="Three validations — card number format, card type (Visa + MasterCard only), expiration date. Returns mock UUID transaction."

QUESTIONS[9]="What are the single points of failure in this architecture? Which service failures would cause a complete outage?"
DIFFICULTIES[9]="5/5"
CATEGORIES[9]="Design Analysis"
GROUND_TRUTHS[9]="Frontend = only true SPOF. ProductCatalogService = near-complete outage. CurrencyService = high blast radius. Must distinguish complete outage vs degraded experience."

QUESTIONS[10]="If I wanted to add a wishlist feature, which services would I need to modify and what new gRPC methods would I need to define?"
DIFFICULTIES[10]="5/5"
CATEGORIES[10]="Design Analysis"
GROUND_TRUTHS[10]="New WishlistService (modeled after CartService), proto changes (AddItem, GetWishlist, RemoveItem RPCs), Frontend modifications, K8s manifests. CartService is the reference."

# ─────────────────────────────────────────────────────────────────────
# Helper functions
# ─────────────────────────────────────────────────────────────────────

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_err()   { echo -e "${RED}[ERROR]${NC} $*"; }
log_phase() { echo -e "\n${BOLD}═══ Phase $1: $2 ═══${NC}\n"; }

cleanup() {
  local exit_code=$?
  log_info "Cleaning up..."

  # Kill site server if running.
  if [[ -n "$SITE_PID" ]] && kill -0 "$SITE_PID" 2>/dev/null; then
    log_info "Stopping site server (PID $SITE_PID)..."
    kill "$SITE_PID" 2>/dev/null || true
    wait "$SITE_PID" 2>/dev/null || true
  fi

  if [[ $exit_code -ne 0 ]]; then
    log_err "Script exited with code $exit_code"
    log_info "Temporary files preserved for debugging:"
    log_info "  Repo:    $REPO_DIR"
    log_info "  Results: $RESULTS_DIR"
  fi
}
trap cleanup EXIT

# Run a command with a timeout. Returns 0 on success, 1 on timeout/failure.
run_with_timeout() {
  local timeout_secs=$1
  shift
  timeout "$timeout_secs" "$@" 2>&1
}

# Escape a string for safe JSON embedding (no python needed).
json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"   # backslash
  s="${s//\"/\\\"}"   # double quote
  s="${s//$'\n'/\\n}" # newline
  s="${s//$'\r'/\\r}" # carriage return
  s="${s//$'\t'/\\t}" # tab
  printf '%s' "$s"
}

# Count JSON array length using grep (fallback when python is unavailable).
json_array_len() {
  local file="$1"
  if [[ -n "$PYTHON_BIN" ]]; then
    "$PYTHON_BIN" -c "import json,sys; print(len(json.load(sys.stdin)))" < "$file" 2>/dev/null || echo "?"
  else
    # Rough count: number of top-level objects.
    grep -c '"file_path"' "$file" 2>/dev/null || echo "?"
  fi
}

# Check if JSON has an "answer" field.
json_has_answer() {
  local file="$1"
  if [[ -n "$PYTHON_BIN" ]]; then
    "$PYTHON_BIN" -c "import json,sys; d=json.load(sys.stdin); print('yes' if d.get('answer') else 'no')" < "$file" 2>/dev/null || echo "?"
  else
    if grep -q '"answer"' "$file" 2>/dev/null; then echo "yes"; else echo "no"; fi
  fi
}

# Count results in API JSON response.
json_result_count() {
  local file="$1"
  if [[ -n "$PYTHON_BIN" ]]; then
    "$PYTHON_BIN" -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('results',[])))" < "$file" 2>/dev/null || echo "?"
  else
    grep -c '"file_path"' "$file" 2>/dev/null || echo "?"
  fi
}

# Wait for the site server to become ready.
wait_for_server() {
  local port=$1
  local max_wait=30
  local waited=0
  while ! curl -s -o /dev/null -w '' "http://localhost:$port/" 2>/dev/null; do
    sleep 1
    waited=$((waited + 1))
    if [[ $waited -ge $max_wait ]]; then
      return 1
    fi
  done
  return 0
}

# ─────────────────────────────────────────────────────────────────────
# Phase 1: Pre-flight checks
# ─────────────────────────────────────────────────────────────────────

preflight() {
  log_phase 1 "Pre-flight Checks"

  local ok=true

  # Required env vars — accept GOOGLE_API_KEY or GEMINI_API_KEY.
  if [[ -z "${GOOGLE_API_KEY:-}" ]]; then
    if [[ -n "${GEMINI_API_KEY:-}" ]]; then
      export GOOGLE_API_KEY="$GEMINI_API_KEY"
      log_ok "GOOGLE_API_KEY set from GEMINI_API_KEY"
    else
      log_err "GOOGLE_API_KEY (or GEMINI_API_KEY) is not set"
      ok=false
    fi
  else
    log_ok "GOOGLE_API_KEY is set"
  fi

  # Python availability (optional — used for nicer result parsing).
  if [[ -n "$PYTHON_BIN" ]]; then
    log_ok "Python found: $PYTHON_BIN"
  else
    log_warn "Python not found — using fallback JSON parsing (some stats will show '?')"
  fi

  # Required tools.
  for tool in go git curl timeout; do
    if command -v "$tool" &>/dev/null; then
      log_ok "$tool found: $(command -v "$tool")"
    else
      log_err "$tool not found on PATH"
      ok=false
    fi
  done

  # Go version.
  local go_version
  go_version=$(go version 2>/dev/null || echo "unknown")
  log_info "Go version: $go_version"

  if [[ "$ok" != "true" ]]; then
    log_err "Pre-flight checks failed. Aborting."
    exit 1
  fi

  log_ok "All pre-flight checks passed"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 2: Build autodoc binary
# ─────────────────────────────────────────────────────────────────────

build_autodoc() {
  log_phase 2 "Build Autodoc"

  AUTODOC_BIN="$PROJECT_ROOT/autodoc"
  # On Windows / MSYS, Go produces .exe
  if [[ "$(go env GOOS)" == "windows" ]]; then
    AUTODOC_BIN="${AUTODOC_BIN}.exe"
  fi

  log_info "Building autodoc from $PROJECT_ROOT ..."
  (cd "$PROJECT_ROOT" && go build -o "$AUTODOC_BIN" .)

  if [[ ! -x "$AUTODOC_BIN" ]] && [[ ! -f "$AUTODOC_BIN" ]]; then
    log_err "Build failed — binary not found at $AUTODOC_BIN"
    exit 1
  fi

  log_ok "Built: $AUTODOC_BIN"
  "$AUTODOC_BIN" version 2>/dev/null || log_warn "autodoc version command failed (non-fatal)"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 3: Clone Online Boutique
# ─────────────────────────────────────────────────────────────────────

clone_repo() {
  log_phase 3 "Clone Online Boutique"

  REPO_DIR=$(mktemp -d "${TMPDIR:-/tmp}/online-boutique-e2e.XXXXXX")
  RESULTS_DIR="$REPO_DIR/_e2e_results"
  mkdir -p "$RESULTS_DIR/cli" "$RESULTS_DIR/api"

  log_info "Cloning $REPO_URL into $REPO_DIR ..."
  git clone --depth 1 "$REPO_URL" "$REPO_DIR/repo"

  local file_count
  file_count=$(find "$REPO_DIR/repo/src" -type f 2>/dev/null | wc -l | tr -d ' ')
  log_ok "Cloned. Source files under src/: $file_count"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 4: Write .autodoc.yml config
# ─────────────────────────────────────────────────────────────────────

write_config() {
  log_phase 4 "Write Config"

  cat > "$REPO_DIR/repo/.autodoc.yml" <<'YAML'
provider: google
model: gemini-2.5-flash
embedding_provider: google
embedding_model: gemini-embedding-001
quality: normal
output_dir: .autodoc
include:
  - "src/**"
  - "protos/**"
exclude:
  - node_modules/**
  - vendor/**
  - .git/**
  - "**/*.min.js"
  - "**/*.lock"
  - "**/go.sum"
  - "**/bin/**"
  - "**/obj/**"
  - "**/*.png"
  - "**/*.jpg"
  - "**/*.svg"
  - helm-chart/**
  - kustomize/**
  - terraform/**
  - docs/**
  - .github/**
max_concurrency: 3
max_cost_usd: 25.0
YAML

  log_ok "Config written to $REPO_DIR/repo/.autodoc.yml"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 5: Cost estimate
# ─────────────────────────────────────────────────────────────────────

estimate_cost() {
  log_phase 5 "Cost Estimate"

  local cost_log="$RESULTS_DIR/cost.log"

  log_info "Running autodoc cost ..."
  (cd "$REPO_DIR/repo" && "$AUTODOC_BIN" cost) 2>&1 | tee "$cost_log"

  if [[ "$SKIP_COST_PROMPT" == "true" ]]; then
    log_info "Skipping cost confirmation (--skip-cost-prompt)"
    return 0
  fi

  echo ""
  read -r -p "Proceed with generation? [Y/n] " confirm
  case "${confirm:-Y}" in
    [Yy]*|"") log_ok "Confirmed — proceeding." ;;
    *)
      log_warn "Aborted by user."
      exit 0
      ;;
  esac
}

# ─────────────────────────────────────────────────────────────────────
# Phase 6: Generate documentation
# ─────────────────────────────────────────────────────────────────────

generate_docs() {
  log_phase 6 "Generate Documentation"

  if [[ "$SKIP_GENERATE" == "true" ]]; then
    log_info "Skipping generation (--skip-generate)"
    if [[ ! -d "$REPO_DIR/repo/.autodoc/docs" ]]; then
      log_err "No existing docs found at $REPO_DIR/repo/.autodoc/docs — cannot skip"
      exit 1
    fi
    return 0
  fi

  local gen_log="$RESULTS_DIR/generate.log"

  log_info "Running autodoc generate -v (this may take several minutes) ..."
  log_info "Log: $gen_log"

  local gen_start
  gen_start=$(date +%s)

  (cd "$REPO_DIR/repo" && run_with_timeout "$GENERATE_TIMEOUT" "$AUTODOC_BIN" generate -v) 2>&1 | tee "$gen_log"
  local gen_exit=${PIPESTATUS[0]}

  local gen_end
  gen_end=$(date +%s)
  local gen_duration=$((gen_end - gen_start))

  if [[ $gen_exit -ne 0 ]]; then
    log_err "autodoc generate failed (exit code $gen_exit) after ${gen_duration}s"
    log_err "Check log: $gen_log"
    exit 1
  fi

  log_ok "Generation completed in ${gen_duration}s"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 7: Verify output artifacts
# ─────────────────────────────────────────────────────────────────────

verify_artifacts() {
  log_phase 7 "Verify Artifacts"

  local autodoc_dir="$REPO_DIR/repo/.autodoc"
  local ok=true

  # Check for docs directory.
  if [[ -d "$autodoc_dir/docs" ]]; then
    local doc_count
    doc_count=$(find "$autodoc_dir/docs" -name '*.md' -type f | wc -l | tr -d ' ')
    log_ok "Docs directory exists: $doc_count markdown files"
  else
    log_err "Missing: $autodoc_dir/docs/"
    ok=false
  fi

  # Check for index.md.
  if [[ -f "$autodoc_dir/docs/index.md" ]]; then
    log_ok "index.md exists ($(wc -l < "$autodoc_dir/docs/index.md" | tr -d ' ') lines)"
  else
    log_err "Missing: $autodoc_dir/docs/index.md"
    ok=false
  fi

  # Check for architecture.md.
  if [[ -f "$autodoc_dir/docs/architecture.md" ]]; then
    log_ok "architecture.md exists ($(wc -l < "$autodoc_dir/docs/architecture.md" | tr -d ' ') lines)"
  else
    log_warn "Missing: $autodoc_dir/docs/architecture.md (non-fatal)"
  fi

  # Check for vector DB.
  if [[ -d "$autodoc_dir/vectordb" ]]; then
    local vdb_size
    vdb_size=$(du -sh "$autodoc_dir/vectordb" 2>/dev/null | cut -f1)
    log_ok "Vector DB exists: $vdb_size"
  else
    log_err "Missing: $autodoc_dir/vectordb/"
    ok=false
  fi

  # Check for state.json.
  if [[ -f "$autodoc_dir/state.json" ]]; then
    log_ok "state.json exists"
  else
    log_warn "Missing: $autodoc_dir/state.json (non-fatal)"
  fi

  if [[ "$ok" != "true" ]]; then
    log_err "Artifact verification failed"
    exit 1
  fi

  log_ok "All critical artifacts present"
}

# ─────────────────────────────────────────────────────────────────────
# Phase 8: CLI queries (Track A — Retrieval)
# ─────────────────────────────────────────────────────────────────────

run_cli_queries() {
  log_phase 8 "CLI Queries (Track A: Retrieval)"

  local failures=0

  for i in $(seq 1 10); do
    local question="${QUESTIONS[$i]}"
    local outfile="$RESULTS_DIR/cli/q${i}.json"

    log_info "Q${i} [${DIFFICULTIES[$i]}]: $question"

    if (cd "$REPO_DIR/repo" && run_with_timeout "$QUERY_TIMEOUT" \
        "$AUTODOC_BIN" query "$question" --json --limit 10) > "$outfile" 2>&1; then
      local count
      count=$(json_array_len "$outfile")
      log_ok "  -> $count results saved to q${i}.json"
    else
      log_warn "  -> Query failed or timed out (saved partial output)"
      failures=$((failures + 1))
    fi
  done

  if [[ $failures -gt 0 ]]; then
    log_warn "$failures of 10 CLI queries had issues"
  else
    log_ok "All 10 CLI queries completed"
  fi
}

# ─────────────────────────────────────────────────────────────────────
# Phase 9 + 10: Site server + API queries (Track B — Synthesis)
# ─────────────────────────────────────────────────────────────────────

run_api_queries() {
  log_phase 9 "Site Server + API Queries (Track B: Synthesis)"

  # First, generate the static site.
  log_info "Generating static site ..."
  (cd "$REPO_DIR/repo" && "$AUTODOC_BIN" site) 2>&1

  # Start the site server in background.
  log_info "Starting site server on port $SITE_PORT ..."
  (cd "$REPO_DIR/repo" && "$AUTODOC_BIN" site --serve --port "$SITE_PORT") &
  SITE_PID=$!

  # Wait for server readiness.
  if wait_for_server "$SITE_PORT"; then
    log_ok "Site server ready (PID $SITE_PID)"
  else
    log_err "Site server failed to start within 30s"
    # Try to capture any error output.
    kill "$SITE_PID" 2>/dev/null || true
    SITE_PID=""
    log_warn "Skipping API queries"
    return 1
  fi

  local failures=0

  for i in $(seq 1 10); do
    local question="${QUESTIONS[$i]}"
    local outfile="$RESULTS_DIR/api/q${i}.json"

    log_info "Q${i} [${DIFFICULTIES[$i]}]: $question"

    local escaped_q
    escaped_q=$(json_escape "$question")
    local payload
    payload=$(printf '{"query": "%s", "limit": 10}' "$escaped_q")

    if run_with_timeout "$QUERY_TIMEOUT" \
        curl -s -X POST "http://localhost:$SITE_PORT/api/search" \
          -H "Content-Type: application/json" \
          -d "$payload" > "$outfile" 2>&1; then
      # Check if response has an answer field.
      local has_answer
      has_answer=$(json_has_answer "$outfile")
      local result_count
      result_count=$(json_result_count "$outfile")
      log_ok "  -> ${result_count} results, LLM answer: ${has_answer}"
    else
      log_warn "  -> API query failed or timed out"
      failures=$((failures + 1))
    fi
  done

  # Stop site server.
  log_info "Stopping site server ..."
  kill "$SITE_PID" 2>/dev/null || true
  wait "$SITE_PID" 2>/dev/null || true
  SITE_PID=""

  if [[ $failures -gt 0 ]]; then
    log_warn "$failures of 10 API queries had issues"
  else
    log_ok "All 10 API queries completed"
  fi
}

# ─────────────────────────────────────────────────────────────────────
# Phase 11: Generate markdown report
# ─────────────────────────────────────────────────────────────────────

generate_report() {
  log_phase 10 "Generate Report"

  cat > "$REPORT_FILE" <<EOF
# Autodoc E2E Test Report — Online Boutique

**Generated:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')
**Target repo:** GoogleCloudPlatform/microservices-demo
**Autodoc binary:** $AUTODOC_BIN

---

## Configuration

\`\`\`yaml
$(cat "$REPO_DIR/repo/.autodoc.yml" 2>/dev/null || echo "(config file not found)")
\`\`\`

## Cost Estimate

\`\`\`
$(cat "$RESULTS_DIR/cost.log" 2>/dev/null || echo "(cost log not found)")
\`\`\`

## Artifacts

| Artifact | Status |
|----------|--------|
EOF

  # Artifact status rows.
  local autodoc_dir="$REPO_DIR/repo/.autodoc"
  for artifact in "docs/index.md" "docs/architecture.md" "vectordb" "state.json"; do
    if [[ -e "$autodoc_dir/$artifact" ]]; then
      echo "| \`$artifact\` | Present |" >> "$REPORT_FILE"
    else
      echo "| \`$artifact\` | **Missing** |" >> "$REPORT_FILE"
    fi
  done

  local doc_count
  doc_count=$(find "$autodoc_dir/docs" -name '*.md' -type f 2>/dev/null | wc -l | tr -d ' ')
  echo "| Total markdown docs | $doc_count |" >> "$REPORT_FILE"

  cat >> "$REPORT_FILE" <<'EOF'

---

## Scoring Rubric

### Track A: Retrieval Quality (per question, 0-10)
- **Relevance (0-4):** How many of top-5 results are actually relevant?
- **Coverage (0-3):** Do results contain the critical files needed?
- **Ranking (0-3):** Is the most important file ranked #1-2?

### Track B: Answer Quality (per question, 0-10)
Scored on accuracy, completeness, specificity per ground truth.

### Aggregate
- Max 100 per track. Grade: 90+ Excellent, 75-89 Good, 60-74 Acceptable, 40-59 Below Average, <40 Poor

---

## Results

EOF

  # Write each question's results.
  for i in $(seq 1 10); do
    cat >> "$REPORT_FILE" <<EOF
### Q${i} (Difficulty ${DIFFICULTIES[$i]}) — ${CATEGORIES[$i]}

**Question:** ${QUESTIONS[$i]}

**Ground Truth:** ${GROUND_TRUTHS[$i]}

#### Track A: Retrieval (\`autodoc query --json\`)

\`\`\`json
$(cat "$RESULTS_DIR/cli/q${i}.json" 2>/dev/null || echo "[]")
\`\`\`

| Criteria   | Score (0-max) | Notes |
|------------|---------------|-------|
| Relevance  | _/4           |       |
| Coverage   | _/3           |       |
| Ranking    | _/3           |       |
| **Total A** | **_/10**     |       |

#### Track B: Synthesis (\`/api/search\`)

EOF

    # Extract the answer from API results if available.
    local api_file="$RESULTS_DIR/api/q${i}.json"
    if [[ -f "$api_file" ]]; then
      local answer
      if [[ -n "$PYTHON_BIN" ]]; then
        answer=$("$PYTHON_BIN" -c "
import json, sys
try:
    d = json.load(sys.stdin)
    print(d.get('answer', '(no LLM answer returned)'))
except:
    print('(failed to parse response)')
" < "$api_file" 2>/dev/null || echo "(error reading file)")
      else
        # Extract answer field with grep/sed fallback.
        answer=$(grep -o '"answer":"[^"]*"' "$api_file" 2>/dev/null | head -1 | sed 's/"answer":"//;s/"$//' || echo "(no LLM answer returned)")
        if [[ -z "$answer" ]]; then
          answer="(no LLM answer returned)"
        fi
      fi

      cat >> "$REPORT_FILE" <<EOF
**LLM Answer:**

> $(echo "$answer" | sed 's/^/> /' | sed '1s/^> > /> /')

<details>
<summary>Raw API response</summary>

\`\`\`json
$(cat "$api_file" 2>/dev/null || echo "{}")
\`\`\`

</details>

EOF
    else
      cat >> "$REPORT_FILE" <<EOF
**LLM Answer:** _(API query not run or failed)_

EOF
    fi

    cat >> "$REPORT_FILE" <<EOF
| Criteria       | Score (0-10) | Notes |
|----------------|--------------|-------|
| Accuracy       | _/10         |       |
| Completeness   | _/10         |       |
| Specificity    | _/10         |       |
| **Total B**    | **_/10**     |       |

---

EOF
  done

  # Summary scoring table.
  cat >> "$REPORT_FILE" <<'EOF'
## Summary Scorecard

| Question | Difficulty | Category | Track A (/10) | Track B (/10) |
|----------|-----------|----------|---------------|---------------|
| Q1       | 1/5       | Basic Facts           | _ | _ |
| Q2       | 2/5       | Basic Facts           | _ | _ |
| Q3       | 2/5       | Architecture          | _ | _ |
| Q4       | 2/5       | Architecture          | _ | _ |
| Q5       | 4/5       | Cross-Service Tracing | _ | _ |
| Q6       | 4/5       | Blast Radius          | _ | _ |
| Q7       | 3/5       | Implementation Detail | _ | _ |
| Q8       | 3/5       | Implementation Detail | _ | _ |
| Q9       | 5/5       | Design Analysis       | _ | _ |
| Q10      | 5/5       | Design Analysis       | _ | _ |
| **Total** |          |                       | **_/100** | **_/100** |

### Grade

| Track | Score | Grade |
|-------|-------|-------|
| A (Retrieval)  | _/100 | _ |
| B (Synthesis)  | _/100 | _ |

> Grades: 90+ Excellent, 75-89 Good, 60-74 Acceptable, 40-59 Below Average, <40 Poor

---

## Notes

- Scores marked `_` are to be filled in manually after reviewing results
- Review each question's results against the ground truth
- Consider partial credit for incomplete but directionally correct answers
EOF

  log_ok "Report written to: $REPORT_FILE"
  echo ""
  log_info "Results directory: $RESULTS_DIR"
  log_info "Target repo:      $REPO_DIR/repo"
}

# ─────────────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────────────

main() {
  echo -e "${BOLD}"
  echo "╔═══════════════════════════════════════════════════════╗"
  echo "║   Autodoc E2E Integration Test — Online Boutique     ║"
  echo "║   11 microservices · 5 languages · 10 questions      ║"
  echo "╚═══════════════════════════════════════════════════════╝"
  echo -e "${NC}"

  preflight
  build_autodoc
  clone_repo
  write_config
  estimate_cost
  generate_docs
  verify_artifacts
  run_cli_queries
  run_api_queries
  generate_report

  echo ""
  echo -e "${BOLD}╔═══════════════════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}║   E2E test complete!                                 ║${NC}"
  echo -e "${BOLD}╚═══════════════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "  Report:  ${GREEN}$REPORT_FILE${NC}"
  echo -e "  Results: ${GREEN}$RESULTS_DIR${NC}"
  echo -e "  Repo:    ${GREEN}$REPO_DIR/repo${NC}"
  echo ""
  echo "  Next: review the report and fill in manual scores."
}

main
