#!/usr/bin/env python3
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.git import repo_root
from lib.comment_rules import (
    DOC_LANGUAGE_EXTENSIONS,
    check_echoes,
    comment_body,
    find_comment_start,
    find_doc_candidates,
    find_trailing_comments,
    is_mechanically_exempt,
    normalize,
    resolve_family,
    scan_comments,
    scan_docstrings,
    trailing_comment_echoes_field,
    validate_doc_shape,
)

WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")

def check_reviewer_scope(path, cwd):
    base = cwd or os.getcwd()
    root = repo_root(base)
    if root is None:
        respond("deny", "BLOCKED: the reviewer agent may only edit BACKLOG.md, "
                "and the repo root could not be resolved to confirm this write is in-scope.")
    target = os.path.realpath(path if os.path.isabs(path) else os.path.join(base, path))
    allowed = os.path.realpath(os.path.join(root, "BACKLOG.md"))
    if target != allowed:
        respond("deny", f"BLOCKED: the reviewer agent may only edit BACKLOG.md, not {path}.")

def handle_pre(payload):
    tool_name = payload.get("tool_name", "")
    if tool_name not in WRITE_TOOLS:
        sys.exit(0)

    tool_input = payload.get("tool_input") or {}
    path = tool_input.get("file_path") or tool_input.get("notebook_path", "")

    if payload.get("agent_type") == "reviewer":
        check_reviewer_scope(path, payload.get("cwd"))

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
    old_comments.update(find_trailing_comments(old_text, family))
    prior = tool_input.get("old_string") or "\n".join(e.get("old_string", "") for e in edits)
    old_comments.update(c[0] for c in scan_comments(prior, family, ext))
    old_comments.update(find_trailing_comments(prior, family))
    new_scanned = scan_comments(new_text, family, ext)
    new_trailing = find_trailing_comments(new_text, family)

    if path.endswith((".py", ".pyi")):
        old_docs = set(scan_docstrings(old_text)) | set(scan_docstrings(tool_input.get("old_string") or ""))
        new_scanned.extend((d, False, False) for d in scan_docstrings(new_text) if d not in old_docs)

    doc_valid_lines = set()
    if path.endswith(DOC_LANGUAGE_EXTENSIONS):
        for candidate, decl_line in find_doc_candidates(new_text, family):
            is_indented = len(decl_line) - len(decl_line.lstrip()) > 0
            ok, _ = validate_doc_shape(comment_body(candidate), decl_line, is_indented)
            if ok:
                doc_valid_lines.add(candidate)

    scope = prior if prior.strip() else old_text
    scope_scanned = scan_comments(scope, family, ext)
    scope_trailing = set(find_trailing_comments(scope, family))

    # scoped to the edit's own prior context, not the whole file — a comment
    # that merely happens to already exist elsewhere is still a fresh echo
    edit_old_comments = set(c[0] for c in scope_scanned)
    echoes = check_echoes(new_text.splitlines(), family, edit_old_comments) - doc_valid_lines

    trailing_echoes = []
    for line in new_text.splitlines():
        idx = find_comment_start(line, family)
        if idx is None or not line[:idx].strip():
            continue
        trailing_text = normalize(line[idx:])
        if trailing_text in scope_trailing:
            continue
        if trailing_comment_echoes_field(line[:idx], line[idx:]):
            trailing_echoes.append(trailing_text)

    echo_reason = ""
    if echoes or trailing_echoes:
        all_echoes = list(echoes) + trailing_echoes
        echo_reason = (f"This comment restates the name of the identifier it documents, in {os.path.basename(path)} — code should be self-documenting:\n"
                + "\n".join(all_echoes)
                + "\n\nThis is a hard block, not a prompt: no comment content the coding agent writes gets past this check. "
                  "Comments are only ever added through the write-comments skill, never inline — invoke `/write-comments` yourself "
                  "(with the diff and your reasoning) if a comment is genuinely warranted; write_comments_validator.py is the only "
                  "thing that can splice one in.")

    added_unallowed = []
    for raw_norm, _, in_manual in new_scanned:
        if raw_norm in old_comments:
            continue
        if raw_norm in doc_valid_lines:
            continue
        if not is_mechanically_exempt(raw_norm, in_manual):
            added_unallowed.append(raw_norm)

    for raw_norm in new_trailing:
        if raw_norm in old_comments:
            continue
        if not is_mechanically_exempt(raw_norm):
            added_unallowed.append(f"{raw_norm} (trailing)")

    added_reason = ""
    if added_unallowed:
        added_reason = (
            f"Comment(s) added to {os.path.basename(path)} are denied — code should be self-documenting:\n"
            + "\n".join(f"  - {c}" for c in added_unallowed)
            + "\n\nOnly a machine directive (// eslint-disable, # noqa, //go:, //nolint:, etc.) passes this check silently, "
              "along with a DOC-shaped godoc comment (name-first, one sentence, on a newly-added exported top-level "
              "declaration in a Go file) — that shape is deterministic enough to check here directly. "
              "Everything else — a banner, a NOTE:/FIXME:/TODO: note, a plain WHY/HACK/FIELD sentence, a step marker, "
              "any other narrative comment — is denied, with no ask.\n"
            + "\nThis is a hard block, not a prompt: no comment content the coding agent writes gets past this check. "
              "Comments are only ever added through the write-comments skill, never inline — invoke `/write-comments` yourself "
              "(with the diff and your reasoning) if a comment is genuinely warranted; write_comments_validator.py is the only "
              "thing that can splice one in."
        )

    deny_reasons = [r for r in (echo_reason, added_reason) if r]
    if deny_reasons:
        respond("deny", "\n\n".join(deny_reasons))

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