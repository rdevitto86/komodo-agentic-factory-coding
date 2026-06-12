#!/usr/bin/env bash

set -euo pipefail

PAYLOAD=$(cat)
CMD=$(echo "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null)

# Only act on `gh pr create` calls
if [[ "$CMD" != *"gh pr create"* ]]; then
  exit 0
fi

OUTPUT=$(echo "$PAYLOAD" | jq -r '.tool_response.output // empty' 2>/dev/null)
PR_URL=$(echo "$OUTPUT" | grep -oE 'https://github\.com/[^[:space:]]+' | head -1)

if [[ -z "$PR_URL" ]]; then
  exit 0
fi

BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
STORY=$(echo "$BRANCH" | grep -oE '[A-Z]+-[0-9]+' | head -1)

echo ""
echo "┌─ PR created ──────────────────────────────────────────┐"
echo "│  $PR_URL"
if [[ -n "$STORY" ]]; then
  echo "│  Story: $STORY"
  echo "│"
  echo "│  Next: /trello status $STORY --move-to \"In Review\""
fi
echo "└───────────────────────────────────────────────────────┘"

# When Trello MCP is configured, uncomment and complete this block:
# if [[ -n "$STORY" && -n "${TRELLO_API_KEY:-}" && -n "${TRELLO_API_TOKEN:-}" ]]; then
#   CARD_ID=$(trello-mcp find-card "$STORY")
#   trello-mcp add-attachment "$CARD_ID" --url "$PR_URL" --name "PR: $PR_URL"
#   trello-mcp move-card "$CARD_ID" --list "In Review"
# fi
