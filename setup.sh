#!/usr/bin/env bash
set -e

# ── Conference-Go Setup ──────────────────────────────────────────────────────

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()    { echo -e "${GREEN}[setup]${NC} $*"; }
warn()    { echo -e "${YELLOW}[warn]${NC}  $*"; }
err()     { echo -e "${RED}[error]${NC} $*" >&2; }

# ── Check dependencies ───────────────────────────────────────────────────────

check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    err "'$1' is not installed. $2"
    exit 1
  fi
}

check_cmd docker  "Install Docker: https://docs.docker.com/get-docker/"
check_cmd docker compose 2>/dev/null || check_cmd docker-compose "Install Docker Compose: https://docs.docker.com/compose/install/"

# ── .env / API key ───────────────────────────────────────────────────────────

if [ -z "$GOOGLE_API_KEY" ]; then
  if [ -f .env ]; then
    # shellcheck disable=SC1091
    source .env
  fi
fi

if [ -z "$GOOGLE_API_KEY" ]; then
  warn "GOOGLE_API_KEY is not set."
  echo ""
  echo "  1. Go to https://aistudio.google.com/ and create an API key."
  echo "  2. Then run one of:"
  echo ""
  echo "       export GOOGLE_API_KEY=your_key_here   # current shell"
  echo "       echo 'GOOGLE_API_KEY=your_key_here' > .env  # persist"
  echo ""
  read -rp "Enter your Google AI Studio API key (or press Enter to skip): " input_key
  if [ -n "$input_key" ]; then
    echo "GOOGLE_API_KEY=$input_key" > .env
    export GOOGLE_API_KEY="$input_key"
    info "Saved to .env"
  else
    warn "Continuing without API key — the server will start in limited mode."
  fi
fi

# ── Embeddings cache ─────────────────────────────────────────────────────────

# Docker bind-mounts a file path; the file must exist or Docker creates a dir.
if [ ! -f embeddings_cache.json ]; then
  info "Creating empty embeddings_cache.json for Docker volume mount..."
  echo "[]" > embeddings_cache.json
fi

# ── Build & start ────────────────────────────────────────────────────────────

info "Building Docker image..."
docker compose build

info "Starting server..."
docker compose up -d

echo ""
info "Server is running at http://localhost:8080"
echo ""
echo "  View logs:  docker compose logs -f"
echo "  Stop:       docker compose down"
echo ""

# ── Cold-start note ──────────────────────────────────────────────────────────

if [ "$(wc -c < embeddings_cache.json)" -lt 100 ]; then
  warn "No embeddings cache found — first startup will call the Gemini API"
  warn "to preprocess and embed all 308 profiles (~86s). Subsequent starts"
  warn "load from disk cache (~24s including Docker build)."
fi
