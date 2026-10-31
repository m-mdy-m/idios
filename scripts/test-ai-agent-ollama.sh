#!/usr/bin/env bash
set -Eeuo pipefail

MODEL="${IDIOS_OLLAMA_MODEL:-qwen2.5:3b}"
BASE_URL="${IDIOS_OLLAMA_BASE_URL:-http://127.0.0.1:11434}"
OLLAMA_HOST="${BASE_URL#http://}"
OLLAMA_HOST="${OLLAMA_HOST#https://}"
export OLLAMA_HOST
export IDIOS_OLLAMA_BASE_URL="$BASE_URL"
export IDIOS_OLLAMA_MODEL="$MODEL"

echo "==> Preparing Ollama for IDIOS local-agent integration"
if ! command -v ollama >/dev/null 2>&1; then
  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required to install Ollama." >&2
    exit 1
  fi
  curl -fsSL https://ollama.com/install.sh | sh
fi

if ! curl --connect-timeout 2 --max-time 5 -fsS "$BASE_URL/api/tags" >/dev/null 2>&1; then
  echo "==> Starting Ollama service"
  ollama serve >"${RUNNER_TEMP:-/tmp}/idios-ollama.log" 2>&1 &
  OLLAMA_PID=$!
  trap 'kill "$OLLAMA_PID" >/dev/null 2>&1 || true' EXIT
fi

ready=0
for attempt in $(seq 1 90); do
  if curl --connect-timeout 2 --max-time 5 -fsS "$BASE_URL/api/tags" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != "1" ]]; then
  echo "Ollama did not become ready at $BASE_URL." >&2
  if [[ -f "${RUNNER_TEMP:-/tmp}/idios-ollama.log" ]]; then
    tail -n 100 "${RUNNER_TEMP:-/tmp}/idios-ollama.log" >&2
  fi
  exit 1
fi

echo "==> Downloading local test model: $MODEL"
ollama pull "$MODEL"
ollama show "$MODEL" >/dev/null

echo "==> Warming the model before the round-table test"
ollama run "$MODEL" "Reply with the single word READY." >/dev/null

echo "==> Running the real Shelf agent discussion against Ollama"
IDIOS_OLLAMA_INTEGRATION=1 go test ./internal/ai \
  -run '^TestOllamaBookDiscussionIntegration$' \
  -count=1 -v -timeout 20m
