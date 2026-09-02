import ast
import os
import re
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

def is_mechanically_exempt(normalized, in_manual=False):
    if normalized.startswith("#!") or in_manual:
        return True
    body = comment_body(normalized)
    if not body:
        return True

    # Check Machine Directives
    body_lower = body.lower()
    if any(body_lower.startswith(p) for p in EXEMPT_PREFIXES):
        return True

    return False

def is_narrative_template(normalized, is_indented=False):
    body = comment_body(normalized)
    if not body:
        return False

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

def check_echoes(lines, family, old_comments=frozenset()):
    """Detect comments where first word matches the function/type name right below it."""
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker: return set()
    echoes = set()

    for i in range(len(lines) - 1):
        curr, nxt = lines[i].strip(), lines[i+1].strip()
        if curr.startswith(line_marker) and curr not in old_comments:
            for pattern in DECL_NAME:
                match = pattern.match(nxt)
                if match:
                    decl_name = match.group(1)
                    body = comment_body(normalize(curr))
                    first_word = body.split()[0].strip("*(),.:;'\"`") if body else ""
                    if first_word.lower() == decl_name.lower():
                        echoes.add(curr)
    return echoes
