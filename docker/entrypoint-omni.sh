#!/bin/bash
set -euo pipefail

# Forward signals to child processes
cleanup() {
  kill -TERM "$OLLAMA_PID" 2>/dev/null || true
  wait "$OLLAMA_PID" 2>/dev/null || true
}
trap cleanup SIGINT SIGTERM

# Ensure volume ownership at runtime
chown -R ollama:ollama /home/ollama/.ollama
chown -R lmgate:lmgate /app/data

# Start Ollama as the ollama user
gosu ollama ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to be ready (max ~30s)
MAX_RETRIES=60
RETRY=0
until curl -sf http://localhost:11434/api/tags > /dev/null 2>&1; do
  RETRY=$((RETRY + 1))
  if [ "$RETRY" -ge "$MAX_RETRIES" ]; then
    echo "ERROR: Ollama failed to start after ${MAX_RETRIES} retries" >&2
    exit 1
  fi
  sleep 0.5
done

# Start LM Gate as the lmgate user (foreground, PID 1 via exec)
exec gosu lmgate /app/lmgate "$@"
