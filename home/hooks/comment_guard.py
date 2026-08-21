#!/usr/bin/env python3
import ast
import json
import os
import re
import sys
import tempfile
import textwrap
from collections import Counter

# --- Language Classification ---
C_FAMILY, HASH_FAMILY, DASH_FAMILY, BLOCK_FAMILY = "c", "hash", "dash", "block"

EXTENSION_FAMILY = {
    ".c": C_FAMILY, ".cc": C_FAMILY, ".cpp": C_FAMILY, ".cs": C_FAMILY, ".dart": C_FAMILY,
    ".go": C_FAMILY, ".h": C_FAMILY, ".hpp": C_FAMILY, ".java": C_FAMILY, ".js": C_FAMILY,
    ".jsx": C_FAMILY, ".kt": C_FAMILY, ".m": C_FAMILY, ".mm": C_FAMILY, ".php": C_FAMILY,
    ".rs": C_FAMILY, ".scala": C_FAMILY, ".swift": C_FAMILY, ".ts": C_FAMILY, ".tsx": C_FAMILY,
    ".zig": C_FAMILY, ".vue": C_FAMILY, ".svelte": C_FAMILY,
    ".css": BLOCK_FAMILY,
    ".bash": HASH_FAMILY, ".sh": HASH_FAMILY, ".py": HASH_FAMILY, ".pyi": HASH_FAMILY,
    ".rb": HASH_FAMILY, ".yaml": HASH_FAMILY, ".yml": HASH_FAMILY, ".toml": HASH_FAMILY,
    ".tf": HASH_FAMILY, ".zsh": HASH_FAMILY,
    ".lua": DASH_FAMILY, ".sql": DASH_FAMILY,
}

FILENAME_FAMILY = {
    "Dockerfile": HASH_FAMILY, "Makefile": HASH_FAMILY, "Justfile": HASH_FAMILY
}

FAMILY_SYNTAX = {
    C_FAMILY: ("//", "/*", "*/", "\"'", "`"),
    BLOCK_FAMILY: (None, "/*", "*/", "\"'", ""),
    HASH_FAMILY: ("#", None, None, "\"'", ""),
    DASH_FAMILY: ("--", "/*", "*/", "'\"", ""),
}

# Extensions whose markup region uses a second comment syntax the family
# above doesn't cover — .vue/.svelte template blocks use HTML comments even
# though their <script> block is C_FAMILY.
EXTRA_BLOCKS = {
    ".vue": [("<!--", "-->")],
    ".svelte": [("<!--", "-->")],
    ".html": [("<!--", "-->")],
}

# --- Directives & Machine Prefix Whitelist ---
EXEMPT_PREFIXES = (
    "!", "+build", "-*- coding", "biome-ignore", "cgo", "clang-format",
    "eslint-disable", "eslint-enable", "fmt:", "go:", "golangci", "isort:",
    "istanbul ignore", "lint:", "mypy:", "nolint", "noqa", "nosec", "pragma",
    "prettier-ignore", "pylint:", "pyright:", "ruff:", "shellcheck",
    "ts-expect-error", "ts-ignore", "ts-nocheck", "type:", "use client", "use server"
)

# --- Allowed Strict Comment Templates ---
TEMPLATE_PATTERNS = [
    re.compile(r"^-{3,}\s*([^\s-][^-]{0,39})\s*-{3,}$"),              # Banner: --- Label ---
    re.compile(r"^(?:WHY|NOTE|FIXME|HACK):\s+\S+"),                   # Intent: WHY: reason
    re.compile(r"^TODO\([a-zA-Z0-9_-]+\):\s+\S+"),                   # Todo: TODO(username): msg
    re.compile(r"^\d+\.\s+\S+"),                                     # Step: 1. Do action
]

STEP_MAX_CHARS = 80
DECL_NAME = (
    re.compile(r"^func\s+\([^)]*\)\s*(\w+)"),
    re.compile(r"^(?:export\s+)?(?:pub\s+)?(?:async\s+)?(?:func|function|def|class|type|struct|enum|fn|const|var|let)\s+(\w+)"),
)
WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")

def resolve_family(path):
    if not path: return None
    base = os.path.basename(path)
    if base in FILENAME_FAMILY: return FILENAME_FAMILY[base]
    _, ext = os.path.splitext(base)
    return EXTENSION_FAMILY.get(ext.lower())

def normalize(raw):
    return " ".join(part.strip() for part in raw.splitlines() if part.strip()).strip()

def comment_body(normalized):
    body = normalized
    for marker in ("///", "//", "/**", "/*", "<!--", "#!", "#", "--"):
        if body.startswith(marker):
            body = body[len(marker):]
            break
    if body.endswith("-->"): body = body[:-3]
    elif body.endswith("*/"): body = body[:-2]
    return body.strip().lstrip("*").strip()

def is_exempt_or_template(normalized, is_indented=False, in_manual=False):
    if normalized.startswith("#!") or in_manual:
        return True
    body = comment_body(normalized)
    if not body:
        return True
    
    # Check Machine Directives
    body_lower = body.lower()
    if any(body_lower.startswith(p) for p in EXEMPT_PREFIXES):
        return True
        
    # Check Regex Templates (WHY:, NOTE:, Banners, TODOs)
    if any(pattern.match(body) for pattern in TEMPLATE_PATTERNS):
        return True

    # Check Step Markers inside function bodies
    if is_indented and len(body) <= STEP_MAX_CHARS:
        return True

    return False

def scan_docstrings(text):
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return []
    kinds = (ast.Module, ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)
    return [normalize(ast.get_docstring(n, clean=False))
            for n in ast.walk(tree)
            if isinstance(n, kinds) and ast.get_docstring(n, clean=False)]


def scan_blocks(text, block_open, block_close):
    found, depth, start = [], 0, 0
    i = 0
    while i < len(text):
        if text.startswith(block_open, i):
            if depth == 0:
                start = i
            depth += 1
            i += len(block_open)
            continue
        if depth and text.startswith(block_close, i):
            depth -= 1
            i += len(block_close)
            if depth == 0:
                found.append(normalize(text[start:i]))
            continue
        i += 1
    return found


def scan_comments(text, family, ext=None):
    line_marker, block_open, block_close, quotes, raw_quotes = FAMILY_SYNTAX[family]
    found = []
    lines = text.splitlines()
    shebang = bool(lines) and lines[0].startswith("#!")
    run = 0

    for index, line in enumerate(lines):
        stripped = line.strip()
        if line_marker and stripped.startswith(line_marker):
            indent = len(line) - len(line.lstrip())
            # a run of comment lines is prose, whatever each line looks like
            run += 1
            in_manual = shebang and run == index + 1
            found.append((normalize(stripped), indent > 0 and run == 1, in_manual))
        else:
            run = 0

    if block_open and block_close:
        found.extend((body, False, False) for body in scan_blocks(text, block_open, block_close))

    # a second markup-comment syntax some C_FAMILY templates also allow
    for extra_open, extra_close in EXTRA_BLOCKS.get(ext, ()):
        found.extend((body, False, False) for body in scan_blocks(text, extra_open, extra_close))

    return found

def check_echoes(lines, family):
    """Detect comments where first word matches the function/type name right below it."""
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker: return set()
    echoes = set()
    
    for i in range(len(lines) - 1):
        curr, nxt = lines[i].strip(), lines[i+1].strip()
        if curr.startswith(line_marker):
            for pattern in DECL_NAME:
                match = pattern.match(nxt)
                if match:
                    decl_name = match.group(1)
                    body = comment_body(normalize(curr))
                    first_word = body.split()[0].strip("*(),.:;'\"`") if body else ""
                    if first_word.lower() == decl_name.lower():
                        echoes.add(curr)
    return echoes

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
    surviving = Counter(c[0] for c in new_scanned)
    removed = []
    for body, count in Counter(c[0] for c in scan_comments(scope, family, ext)).items():
        if count > surviving.get(body, 0):
            removed.append(body)

    if removed:
        respond("ask", "This edit removes comment(s) it did not add:\n"
                + "\n".join(f"  - {c}" for c in removed)
                + "\n\nApprove only if the removal is intended. Moving code? "
                  "Re-add each line verbatim at the destination.")

    echoes = check_echoes(new_text.splitlines(), family)
    if echoes:
        respond("ask", f"This comment restates the name of the identifier below it, in {os.path.basename(path)} — code should be self-documenting:\n"
                + "\n".join(echoes)
                + "\n\nDirectives are provisional; approve only if this one is actually wanted.")

    added_unallowed = []
    for raw_norm, is_indented, in_manual in new_scanned:
        if raw_norm in old_comments:
            continue
        if not is_exempt_or_template(raw_norm, is_indented, in_manual):
            added_unallowed.append(raw_norm)

    if added_unallowed:
        reason = (
            f"Comment(s) added to {os.path.basename(path)} don't match an allowed style — code should be self-documenting:\n"
            + "\n".join(f"  - {c}" for c in added_unallowed)
            + "\n\nAllowed styles:\n"
            + "  - Machine directives (// eslint-disable, # noqa, //go:, //nolint:)\n"
            + "  - Banners (// --- Title ---)\n"
            + "  - Structured Notes (// WHY: ..., // NOTE: ..., // TODO(user): ...)\n"
            + "  - Short indented step markers (// 1. Process batch)\n"
            + "\nDirectives are provisional — approve if this one is actually wanted, deny if not."
        )
        respond("ask", reason)

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