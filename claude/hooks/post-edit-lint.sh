#!/usr/bin/env bash
# Hook: PostToolUse(Edit|Write)
# Runs the appropriate linter after Claude edits a file.
# Receives tool context on stdin as JSON.
# Debounces to avoid re-linting the same project within DEBOUNCE_SEC.

set -euo pipefail

DEBOUNCE_SEC=30

PAYLOAD=$(cat)
FILE=$(echo "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null)

if [[ -z "$FILE" || ! -f "$FILE" ]]; then
  exit 0
fi

# Debounce: skip if we linted the same project within DEBOUNCE_SEC.
debounce_check() {
  local key="$1"
  local stamp="/tmp/.komodo-lint-$(echo -n "$key" | shasum | awk '{print $1}')"
  local now
  now=$(date +%s)
  if [[ -f "$stamp" ]]; then
    local last
    last=$(cat "$stamp")
    if (( now - last < DEBOUNCE_SEC )); then
      return 1
    fi
  fi
  echo "$now" > "$stamp"
  return 0
}

if [[ "$FILE" == *.go ]]; then
  PKG_DIR=$(dirname "$FILE")
  if ! command -v golangci-lint &> /dev/null; then
    exit 0
  fi
  if ! debounce_check "go:$PKG_DIR"; then
    exit 0
  fi
  echo "→ lint: $PKG_DIR"
  # Keep the most recent errors (tail) — partial outputs put the relevant context at the bottom.
  golangci-lint run --fast "$PKG_DIR/..." 2>&1 | tail -40 || true

elif [[ "$FILE" == *.ts || "$FILE" == *.tsx || "$FILE" == *.svelte ]]; then
  ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "")
  if [[ -z "$ROOT" ]]; then
    exit 0
  fi
  DIR=$(dirname "$FILE")
  TSCONFIG=""
  while [[ "$DIR" != "$ROOT" && "$DIR" != "/" ]]; do
    if [[ -f "$DIR/tsconfig.json" ]]; then
      TSCONFIG="$DIR/tsconfig.json"
      break
    fi
    DIR=$(dirname "$DIR")
  done
  if [[ -z "$TSCONFIG" && -f "$ROOT/tsconfig.json" ]]; then
    TSCONFIG="$ROOT/tsconfig.json"
  fi
  if [[ -z "$TSCONFIG" ]]; then
    exit 0
  fi
  TSROOT=$(dirname "$TSCONFIG")
  if ! debounce_check "ts:$TSROOT"; then
    exit 0
  fi
  echo "→ tsc: $TSROOT"
  cd "$TSROOT" && tsc --noEmit 2>&1 | tail -40 || true
fi
