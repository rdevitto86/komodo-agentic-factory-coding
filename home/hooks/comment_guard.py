#!/usr/bin/env python3

import ast
import json
import os
import re
import sys
import tempfile
import textwrap
from collections import Counter

C_FAMILY = "c"
HASH_FAMILY = "hash"
DASH_FAMILY = "dash"
BLOCK_FAMILY = "block"

EXTENSION_FAMILY = {
    ".c": C_FAMILY,
    ".cc": C_FAMILY,
    ".cjs": C_FAMILY,
    ".cpp": C_FAMILY,
    ".cs": C_FAMILY,
    ".dart": C_FAMILY,
    ".go": C_FAMILY,
    ".gradle": C_FAMILY,
    ".groovy": C_FAMILY,
    ".h": C_FAMILY,
    ".hh": C_FAMILY,
    ".hpp": C_FAMILY,
    ".java": C_FAMILY,
    ".js": C_FAMILY,
    ".jsx": C_FAMILY,
    ".kt": C_FAMILY,
    ".kts": C_FAMILY,
    ".less": C_FAMILY,
    ".m": C_FAMILY,
    ".mjs": C_FAMILY,
    ".mm": C_FAMILY,
    ".php": C_FAMILY,
    ".proto": C_FAMILY,
    ".rs": C_FAMILY,
    ".sass": C_FAMILY,
    ".scala": C_FAMILY,
    ".scss": C_FAMILY,
    ".svelte": C_FAMILY,
    ".swift": C_FAMILY,
    ".ts": C_FAMILY,
    ".tsx": C_FAMILY,
    ".vue": C_FAMILY,
    ".zig": C_FAMILY,
    ".css": BLOCK_FAMILY,
    ".bash": HASH_FAMILY,
    ".conf": HASH_FAMILY,
    ".env": HASH_FAMILY,
    ".fish": HASH_FAMILY,
    ".hcl": HASH_FAMILY,
    ".ini": HASH_FAMILY,
    ".mk": HASH_FAMILY,
    ".pl": HASH_FAMILY,
    ".py": HASH_FAMILY,
    ".pyi": HASH_FAMILY,
    ".r": HASH_FAMILY,
    ".rb": HASH_FAMILY,
    ".sh": HASH_FAMILY,
    ".tf": HASH_FAMILY,
    ".tfvars": HASH_FAMILY,
    ".toml": HASH_FAMILY,
    ".yaml": HASH_FAMILY,
    ".yml": HASH_FAMILY,
    ".zsh": HASH_FAMILY,
    ".lua": DASH_FAMILY,
    ".sql": DASH_FAMILY,
}

FILENAME_FAMILY = {
    "Dockerfile": HASH_FAMILY,
    "Makefile": HASH_FAMILY,
    "Justfile": HASH_FAMILY,
    "Rakefile": HASH_FAMILY,
    "Gemfile": HASH_FAMILY,
    "Vagrantfile": HASH_FAMILY,
}

FAMILY_SYNTAX = {
    C_FAMILY: ("//", "/*", "*/", "\"'", "`"),
    BLOCK_FAMILY: (None, "/*", "*/", "\"'", ""),
    HASH_FAMILY: ("#", None, None, "\"'", ""),
    DASH_FAMILY: ("--", "/*", "*/", "'\"", ""),
}

EXEMPT_PREFIXES = (
    "!",
    "+build",
    "-*- coding",
    "biome-ignore",
    "cgo",
    "checkstyle",
    "clang-format",
    "code generated",
    "coverage:",
    "deno-lint-ignore",
    "endregion",
    "eslint-disable",
    "eslint-enable",
    "fmt:",
    "go:",
    "golangci",
    "isort:",
    "istanbul ignore",
    "jshint",
    "line ",
    "lint:",
    "mypy:",
    "noinspection",
    "nolint",
    "noqa",
    "nosec",
    "pragma",
    "prettier-ignore",
    "pylint:",
    "pyright:",
    "region",
    "ruff:",
    "shellcheck",
    "skipcq",
    "sourcery",
    "spdx-",
    "swagger:",
    "ts-expect-error",
    "ts-ignore",
    "ts-nocheck",
    "type:",
    "v8 ignore",
    "yapf:",
)

EXEMPT_SUBSTRINGS = (
    "@ts-expect-error",
    "@ts-ignore",
    "@ts-nocheck",
    "biome-ignore",
    "eslint-disable",
    "eslint-enable",
    "istanbul ignore",
    "prettier-ignore",
    "spdx-license-identifier",
    "v8 ignore",
)

STEP_MAX_CHARS = 80
BANNER_MAX_LABEL = 40
BANNER = re.compile(r"^-{3,}\s*([^\s-][^-]{0,%d})\s*-{3,}$" % (BANNER_MAX_LABEL - 1))
DECL_NAME = (
    re.compile(r"^func\s+\([^)]*\)\s*(\w+)"),
    re.compile(
        r"^(?:export\s+)?(?:default\s+)?(?:public\s+|private\s+|protected\s+)?"
        r"(?:static\s+)?(?:abstract\s+)?(?:readonly\s+)?(?:async\s+)?"
        r"(?:pub(?:\([^)]*\))?\s+)?"
        r"(?:func|function|def|class|type|const|var|let|interface|struct|enum|impl|fn|trait)"
        r"\s+(\w+)"
    ),
)
WRITE_TOOLS = ("Edit", "Write", "MultiEdit", "NotebookEdit")
SAFE_NAME = re.compile(r"[^A-Za-z0-9._-]")
GRANT_SIGIL = re.compile(r"(?<![A-Za-z0-9_])\+comments(?![A-Za-z0-9_])", re.IGNORECASE)
PROMPT_EVENTS = ("UserPromptSubmit", "SessionStart")
LEDGER_LIMIT = 400


def resolve_family(path):
    if not path:
        return None
    base = os.path.basename(path)
    if base in FILENAME_FAMILY:
        return FILENAME_FAMILY[base]
    for name, family in FILENAME_FAMILY.items():
        if base.startswith(name + "."):
            return family
    _, ext = os.path.splitext(base)
    return EXTENSION_FAMILY.get(ext.lower())


def comment_runs(text, family):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return []
    lines = text.splitlines()
    total = len(lines)
    runs = []
    index = 0
    while index < total:
        if not lines[index].strip().startswith(line_marker):
            index += 1
            continue
        start = index
        while index < total and lines[index].strip().startswith(line_marker):
            index += 1
        following = lines[index].strip() if index < total else ""
        runs.append((lines[start:index], following))
    return runs


def declared_name(line):
    for pattern in DECL_NAME:
        match = pattern.match(line)
        if match:
            return match.group(1)
    return None


def slot_comments(text, family):
    found = set()
    for run, _following in comment_runs(text, family):
        if len(run) != 1:
            continue
        raw = run[0]
        body = comment_body(normalize(raw))
        if not body:
            continue
        if BANNER.match(body):
            found.add(normalize(raw))
            continue
        if raw[:1] in (" ", "\t") and len(body) <= STEP_MAX_CHARS:
            found.add(normalize(raw))
    return found


def name_echoes(text, family):
    found = set()
    for run, following in comment_runs(text, family):
        name = declared_name(following)
        if not name:
            continue
        body = comment_body(normalize(run[0]))
        first = body.split(" ")[0].strip("*(),.:;'\"`") if body else ""
        if first and first == name:
            found.add(normalize(run[0]))
    return found


def scan_comments(text, family):
    line_marker, block_open, block_close, quotes, raw_quotes = FAMILY_SYNTAX[family]
    require_boundary = family == HASH_FAMILY
    found = []
    index = 0
    length = len(text)
    while index < length:
        char = text[index]
        if raw_quotes and char in raw_quotes:
            closing = text.find(char, index + 1)
            index = length if closing == -1 else closing + 1
            continue
        if char in quotes:
            index += 1
            while index < length:
                if text[index] == "\\":
                    index += 2
                    continue
                if text[index] == char:
                    index += 1
                    break
                if text[index] == "\n":
                    break
                index += 1
            continue
        if block_open and text.startswith(block_open, index):
            closing = text.find(block_close, index + len(block_open))
            end = length if closing == -1 else closing + len(block_close)
            found.append(text[index:end])
            index = end
            continue
        if line_marker and text.startswith(line_marker, index):
            if require_boundary and index > 0 and text[index - 1] not in " \t\n":
                index += 1
                continue
            newline = text.find("\n", index)
            end = length if newline == -1 else newline
            found.append(text[index:end])
            index = end
            continue
        index += 1
    return found


def triple_quote_docstrings(text):
    found = []
    for quote in ('"""', "'''"):
        index = 0
        while True:
            start = text.find(quote, index)
            if start == -1:
                break
            closing = text.find(quote, start + 3)
            end = len(text) if closing == -1 else closing + 3
            line_start = text.rfind("\n", 0, start) + 1
            if not text[line_start:start].strip():
                found.append(text[start:end])
            index = end
    return found


def ast_docstrings(text):
    for candidate in (text, textwrap.dedent(text)):
        try:
            tree = ast.parse(candidate)
        except (SyntaxError, ValueError):
            continue
        found = []
        for node in ast.walk(tree):
            if isinstance(node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
                doc = ast.get_docstring(node, clean=False)
                if doc is not None:
                    found.append(doc)
        return found
    return None


def python_docstrings(text):
    exact = ast_docstrings(text)
    if exact is not None:
        return [normalize(item) for item in exact]
    return [normalize(item) for item in triple_quote_docstrings(text)]


def normalize(raw):
    return " ".join(part.strip() for part in raw.splitlines() if part.strip()).strip()


def comment_body(normalized):
    body = normalized
    for marker in ("///", "//", "/**", "/*", "#!", "#", "--"):
        if body.startswith(marker):
            body = body[len(marker):]
            break
    if body.endswith("*/"):
        body = body[:-2]
    return body.strip().lstrip("*").strip()


def is_exempt(normalized):
    if normalized.startswith("#!"):
        return True
    lowered = normalized.lower()
    for needle in EXEMPT_SUBSTRINGS:
        if needle in lowered:
            return True
    body = comment_body(normalized).lower()
    if not body:
        return True
    for prefix in EXEMPT_PREFIXES:
        if body.startswith(prefix):
            return True
    return False


def state_path(kind, session_id):
    token = SAFE_NAME.sub("_", session_id or "unknown")
    return os.path.join(tempfile.gettempdir(), "claude-comment-%s-%s" % (kind, token))


def grant_path(session_id):
    return state_path("grant", session_id)


def has_grant(session_id):
    return os.path.exists(grant_path(session_id))


def ledger_load(session_id):
    try:
        with open(state_path("ledger", session_id), "r", encoding="utf-8") as handle:
            return set(line.rstrip("\n") for line in handle if line.strip())
    except OSError:
        return set()


def ledger_add(session_id, items):
    existing = ledger_load(session_id)
    merged = list(existing) + [item for item in items if item not in existing]
    try:
        with open(state_path("ledger", session_id), "w", encoding="utf-8") as handle:
            for item in merged[-LEDGER_LIMIT:]:
                handle.write(item + "\n")
    except OSError:
        pass


def header_comments(text, family):
    if not text.startswith("#!"):
        return set()
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return set()
    found = set()
    for line in text.splitlines()[1:]:
        stripped = line.strip()
        if not stripped or not stripped.startswith(line_marker):
            break
        found.add(normalize(stripped))
    return found


def extract(text, family, path):
    if not text:
        return []
    items = [normalize(raw) for raw in scan_comments(text, family)]
    if path.lower().endswith((".py", ".pyi")):
        items.extend(python_docstrings(text))
    return [item for item in items if item]


def compare(old_text, new_text, family, path, moved):
    old_counts = Counter(extract(old_text, family, path))
    new_counts = Counter(extract(new_text, family, path))
    added = list((new_counts - old_counts).elements())
    removed = list((old_counts - new_counts).elements())
    header = header_comments(new_text, family)
    slots = slot_comments(new_text, family)
    echoed = name_echoes(new_text, family)
    echoes = [item for item in added if item in echoed and not is_exempt(item)]
    added = [
        item
        for item in added
        if not is_exempt(item)
        and item not in header
        and item not in slots
        and item not in moved
        and item not in echoed
    ]
    removed = [item for item in removed if not is_exempt(item)]
    return added, removed, echoes


def read_file(path):
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except (OSError, UnicodeError):
        return ""


def notebook_cell_source(path, cell_id):
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as handle:
            notebook = json.load(handle)
    except (OSError, ValueError):
        return ""
    for cell in notebook.get("cells", []):
        if cell.get("id") == cell_id:
            source = cell.get("source", "")
            return "".join(source) if isinstance(source, list) else source
    return ""


def pairs_for(tool_name, tool_input):
    if tool_name == "Edit":
        path = tool_input.get("file_path", "")
        return path, [(tool_input.get("old_string", ""), tool_input.get("new_string", ""))]
    if tool_name == "Write":
        path = tool_input.get("file_path", "")
        return path, [(read_file(path), tool_input.get("content", ""))]
    if tool_name == "MultiEdit":
        path = tool_input.get("file_path", "")
        edits = tool_input.get("edits") or []
        return path, [(edit.get("old_string", ""), edit.get("new_string", "")) for edit in edits]
    if tool_name == "NotebookEdit":
        path = tool_input.get("notebook_path", "") or tool_input.get("file_path", "")
        old = notebook_cell_source(path, tool_input.get("cell_id"))
        return path, [(old, tool_input.get("new_source", ""))]
    return "", []


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


def allow():
    sys.exit(0)


def format_echo_reason(path, echoes):
    lines = [
        "BLOCKED. A comment restates the name of the thing it sits above.",
        "",
        "In %s:" % os.path.basename(path),
    ]
    lines.extend("    %s" % item for item in echoes)
    lines.extend([
        "",
        "A comment whose first word is the identifier declared on the next line",
        "carries no information. Delete it; do not reword it.",
        "",
        "+comments does NOT lift this rule.",
    ])
    return "\n".join(lines)


def format_addition_reason(path, added):
    lines = [
        "BLOCKED. Nothing was written to disk.",
        "",
        "New comment(s) in %s:" % os.path.basename(path),
    ]
    lines.extend("    %s" % item for item in added)
    lines.extend([
        "",
        "No docs on func, type, const, var, package, or struct. No block comments,",
        "docstrings, or JSDoc. Code must be self-documenting.",
        "",
        "Three forms are allowed, all single-line:",
        "    step marker   indented, inside a body, %d chars or fewer" % STEP_MAX_CHARS,
        "    section break --- Label --- with a label of %d chars or fewer" % BANNER_MAX_LABEL,
        "    script manual line comments directly under a #! shebang, any length",
        "Machine directives (go:, nolint, eslint-disable, noqa, ...) are always exempt.",
        "",
        "Remove the comment text and retry the same edit.",
        "Do NOT resolve this by deleting any other comment in the file.",
        "",
        "Moving existing code? Delete it from the source file FIRST. Approving that",
        "deletion records the comment for this session and lets you re-add it here",
        "verbatim. Adding at the destination before deleting at the source fails.",
        "",
        "Only the user can lift this, by sending a message containing +comments.",
        "Do not ask them to; if they wanted comments they would have said so.",
    ])
    return "\n".join(lines)


def format_removal_reason(path, removed):
    lines = [
        "This edit deletes comment(s) in %s that you did not author." % os.path.basename(path),
        "",
    ]
    lines.extend("    %s" % item for item in removed)
    lines.extend([
        "",
        "Approve only if you intended to remove them. Deny to keep them verbatim,",
        "then re-apply the change with the comment lines left untouched.",
        "",
        "If you are moving this code to another file, approve: the exact text above",
        "is recorded for this session and may be re-added verbatim at the new site.",
        "Reworded text will not be accepted.",
    ])
    return "\n".join(lines)


def handle_prompt(payload):
    location = grant_path(payload.get("session_id"))
    if GRANT_SIGIL.search(payload.get("prompt") or ""):
        try:
            with open(location, "w", encoding="utf-8") as handle:
                handle.write("granted")
        except OSError:
            pass
        allow()
    try:
        os.remove(location)
    except OSError:
        pass
    allow()


def handle_pre(payload):
    tool_name = payload.get("tool_name", "")
    if tool_name not in WRITE_TOOLS:
        allow()
    tool_input = payload.get("tool_input") or {}
    path, pairs = pairs_for(tool_name, tool_input)
    family = resolve_family(path)
    if family is None or not pairs:
        allow()
    session_id = payload.get("session_id")
    moved = ledger_load(session_id)
    added = []
    removed = []
    echoes = []
    for old_text, new_text in pairs:
        pair_added, pair_removed, pair_echoes = compare(old_text, new_text, family, path, moved)
        added.extend(pair_added)
        removed.extend(pair_removed)
        echoes.extend(pair_echoes)
    if echoes:
        respond("deny", format_echo_reason(path, echoes))
    if added and not has_grant(session_id):
        respond("deny", format_addition_reason(path, added))
    if removed:
        ledger_add(session_id, removed)
        respond("ask", format_removal_reason(path, removed))
    allow()


def main():
    payload = json.loads(sys.stdin.read())
    event = payload.get("hook_event_name") or "PreToolUse"
    if event in PROMPT_EVENTS:
        handle_prompt(payload)
    if event != "PreToolUse":
        allow()
    handle_pre(payload)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except BaseException as error:
        respond(
            "deny",
            "comment guard failed to evaluate this edit: %s: %s\n"
            "Failing closed. Fix %s before editing code files."
            % (type(error).__name__, error, os.path.abspath(__file__)),
        )
