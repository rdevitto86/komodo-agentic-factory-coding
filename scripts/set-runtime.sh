#!/usr/bin/env bash

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_JSON="$REPO_DIR/runtime.json"

usage() {
  cat <<'EOF'
Usage:
  set-runtime.sh status
  set-runtime.sh local|claude [--agent AGENT_NAME]
  set-runtime.sh clear --agent AGENT_NAME
EOF
}

if ! command -v jq >/dev/null 2>&1; then
  echo "set-runtime: jq is required but not found on PATH." >&2
  exit 1
fi

if [ ! -f "$RUNTIME_JSON" ]; then
  echo "set-runtime: $RUNTIME_JSON not found." >&2
  exit 1
fi

valid_agents() {
  find "$REPO_DIR/agents" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort
}

require_valid_agent() {
  local name="$1"
  if ! valid_agents | grep -qx "$name"; then
    echo "set-runtime: unknown agent '$name'. Valid agents:" >&2
    valid_agents | sed 's/^/  /' >&2
    exit 1
  fi
}

cmd="${1:-}"
[ -z "$cmd" ] && { usage; exit 1; }
shift || true

agent=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)
      agent="${2:-}"
      shift 2
      ;;
    *)
      echo "set-runtime: unrecognized argument '$1'" >&2
      usage
      exit 1
      ;;
  esac
done

case "$cmd" in
  status)
    default_runtime="$(jq -r '.default_runtime' "$RUNTIME_JSON")"
    echo "default_runtime: $default_runtime"
    override_count="$(jq '.overrides | length' "$RUNTIME_JSON")"
    if [ "$override_count" -eq 0 ]; then
      echo "overrides: none"
    else
      echo "overrides:"
      jq -r '.overrides | to_entries[] | "  \(.key): \(.value)"' "$RUNTIME_JSON"
    fi
    ;;

  local|claude)
    if [ -n "$agent" ]; then
      require_valid_agent "$agent"
      tmp="$(mktemp)"
      jq --arg agent "$agent" --arg runtime "$cmd" \
        '.overrides[$agent] = $runtime' "$RUNTIME_JSON" >"$tmp"
      mv "$tmp" "$RUNTIME_JSON"
      echo "set-runtime: override set — $agent -> $cmd"
    else
      tmp="$(mktemp)"
      jq --arg runtime "$cmd" '.default_runtime = $runtime' "$RUNTIME_JSON" >"$tmp"
      mv "$tmp" "$RUNTIME_JSON"
      echo "set-runtime: default_runtime -> $cmd"
    fi
    ;;

  clear)
    if [ -z "$agent" ]; then
      echo "set-runtime: 'clear' requires --agent AGENT_NAME" >&2
      exit 1
    fi
    require_valid_agent "$agent"
    tmp="$(mktemp)"
    jq --arg agent "$agent" 'del(.overrides[$agent])' "$RUNTIME_JSON" >"$tmp"
    mv "$tmp" "$RUNTIME_JSON"
    echo "set-runtime: override cleared for $agent, falls back to default_runtime"
    ;;

  *)
    echo "set-runtime: unrecognized command '$cmd'" >&2
    usage
    exit 1
    ;;
esac
