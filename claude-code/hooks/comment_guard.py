#!/usr/bin/env python3
import json
import os
import sys
from collections import Counter

from lib.comment_rules import (
    check_echoes,
    is_mechanically_exempt,
    resolve_family,
    scan_comments,
    scan_docstrings,
)

WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")

def handle_pre(payload):
    tool_name = payload.get("tool_name", "")
    if tool_name not in WRITE_TOOLS:
        sys.exit(0)

    tool_input = payload.get("tool_input") or {}
    path = tool_input.get("file_path") or tool_input.get("notebook_path", "")
    family = resolve_family(path)
    if not family:
        sys.exit(0)
    ext = os.path.splitext(os.path.basename(path))[1].lower()

    edits = tool_input.get("edits") or []
    new_text = tool_input.get("content") or tool_input.get("new_string") or ""
    if edits:
        new_text = "\n".join(e.get("new_string", "") for e in edits)
    old_text = ""
    if os.path.exists(path):
        with open(path, "r", encoding="utf-8", errors="ignore") as f:
            old_text = f.read()

    old_comments = set(c[0] for c in scan_comments(old_text, family, ext))
    prior = tool_input.get("old_string") or "\n".join(e.get("old_string", "") for e in edits)
    old_comments.update(c[0] for c in scan_comments(prior, family, ext))
    new_scanned = scan_comments(new_text, family, ext)

    if path.endswith((".py", ".pyi")):
        old_docs = set(scan_docstrings(old_text)) | set(scan_docstrings(tool_input.get("old_string") or ""))
        new_scanned.extend((d, False, False) for d in scan_docstrings(new_text) if d not in old_docs)

    scope = prior if prior.strip() else old_text
    scope_scanned = scan_comments(scope, family, ext)
    surviving = Counter(c[0] for c in new_scanned)
    removed = []
    for body, count in Counter(c[0] for c in scope_scanned).items():
        if count > surviving.get(body, 0):
            removed.append(body)

    if removed:
        respond("ask", "This edit removes comment(s) it did not add:\n"
                + "\n".join(f"  - {c}" for c in removed)
                + "\n\nApprove only if the removal is intended. Moving code? "
                  "Re-add each line verbatim at the destination.")

    # scoped to the edit's own prior context, not the whole file — a comment
    # that merely happens to already exist elsewhere is still a fresh echo
    edit_old_comments = set(c[0] for c in scope_scanned)
    echoes = check_echoes(new_text.splitlines(), family, edit_old_comments)
    if echoes:
        respond("deny", f"This comment restates the name of the identifier below it, in {os.path.basename(path)} — code should be self-documenting:\n"
                + "\n".join(echoes)
                + "\n\nThis is a hard block, not a prompt: no comment content the coding agent writes gets past this check. "
                  "Comments are only ever added through the write-comments skill (not yet built as of this task — for now there is no path to add a narrative comment inline).")

    added_unallowed = []
    for raw_norm, is_indented, in_manual in new_scanned:
        if raw_norm in old_comments:
            continue
        if not is_mechanically_exempt(raw_norm, in_manual):
            added_unallowed.append(raw_norm)

    if added_unallowed:
        reason = (
            f"Comment(s) added to {os.path.basename(path)} are denied — code should be self-documenting:\n"
            + "\n".join(f"  - {c}" for c in added_unallowed)
            + "\n\nOnly a machine directive (// eslint-disable, # noqa, //go:, //nolint:, etc.) passes this check silently. "
              "Everything else — a banner, a WHY:/NOTE:/FIXME:/HACK:/TODO(user): note, a step marker, any other narrative "
              "comment — is denied, with no ask.\n"
            + "\nThis is a hard block, not a prompt: no comment content the coding agent writes gets past this check. "
              "Comments are only ever added through the write-comments skill (not yet built as of this task — for now there is no path to add a narrative comment inline)."
        )
        respond("deny", reason)

    sys.exit(0)

def respond(decision, reason):
    payload = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
            "permissionDecisionReason": reason,
        }
    }
    sys.stdout.write(json.dumps(payload))
    sys.exit(0)

if __name__ == "__main__":
    try:
        raw_input = sys.stdin.read()
        if raw_input:
            payload = json.loads(raw_input)
            if payload.get("hook_event_name") == "PreToolUse":
                handle_pre(payload)
    except Exception:
        respond("deny", "BLOCKED: the comment guard could not read this payload.\nFailing closed on purpose — an unparseable payload is not proof the write is clean.")
    sys.exit(0)