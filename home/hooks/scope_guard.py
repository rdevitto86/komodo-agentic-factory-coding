#!/usr/bin/env python3

import json
import os
import re
import sys
import tempfile

WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")
SAFE_NAME = re.compile(r"[^A-Za-z0-9._-]")


def state_path(session_id):
    token = SAFE_NAME.sub("_", session_id or "unknown")
    return os.path.join(tempfile.gettempdir(), "claude-scope-%s.json" % token)


def load_touched(path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, ValueError):
        return []
    return data if isinstance(data, list) else []


def save_touched(path, touched):
    try:
        with open(path, "w", encoding="utf-8") as handle:
            json.dump(touched, handle)
    except OSError:
        pass


def target_path(tool_input):
    return tool_input.get("file_path") or tool_input.get("notebook_path") or ""


def respond_ask(reason):
    payload = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "ask",
            "permissionDecisionReason": reason,
        }
    }
    sys.stdout.write(json.dumps(payload))
    sys.exit(0)


def handle_pre(payload):
    if payload.get("tool_name") not in WRITE_TOOLS:
        sys.exit(0)
    path = target_path(payload.get("tool_input") or {})
    if not path:
        sys.exit(0)
    touched = load_touched(state_path(payload.get("session_id")))
    if not touched or path in touched:
        sys.exit(0)
    already = "\n".join("    %s" % os.path.basename(item) for item in touched)
    respond_ask(
        "This turn already edited:\n%s\n\nThis would also edit:\n    %s\n\n"
        "One file per turn unless you say otherwise. Approve to widen the scope,\n"
        "or deny and the change stays confined to the file above." % (already, path)
    )


def handle_post(payload):
    if payload.get("tool_name") not in WRITE_TOOLS:
        sys.exit(0)
    path = target_path(payload.get("tool_input") or {})
    if not path:
        sys.exit(0)
    location = state_path(payload.get("session_id"))
    touched = load_touched(location)
    if path not in touched:
        touched.append(path)
        save_touched(location, touched)
    sys.exit(0)


def handle_reset(payload):
    try:
        os.remove(state_path(payload.get("session_id")))
    except OSError:
        pass
    sys.exit(0)


HANDLERS = {
    "PreToolUse": handle_pre,
    "PostToolUse": handle_post,
    "UserPromptSubmit": handle_reset,
    "SessionStart": handle_reset,
}


def main():
    payload = json.loads(sys.stdin.read())
    handler = HANDLERS.get(payload.get("hook_event_name"))
    if handler is None:
        sys.exit(0)
    handler(payload)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException:
        sys.exit(0)
