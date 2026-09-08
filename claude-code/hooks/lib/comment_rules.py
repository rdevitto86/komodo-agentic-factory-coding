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
BANNER_LABEL = "Setup"
TEMPLATE_PATTERNS = {
    "BANNER": re.compile(r"^-{3,}\s*" + re.escape(BANNER_LABEL) + r"\s*-{3,}$"),
    "NOTE": re.compile(r"^NOTE:\s+\S+"),
    "FIXME": re.compile(r"^FIXME:\s+\S+"),
    "TODO": re.compile(r"^TODO:\s+\S+"),
    "STEP": re.compile(r"^\d+\.\s+\S+"),
}
BANNER_SHAPE = re.compile(r"^-{3,}")
RESERVED_MARKERS = ("NOTE:", "FIXME:", "TODO:", "WHY:", "HACK:")

STEP_MAX_CHARS = 80
NARRATIVE_MAX_CHARS = 120
DOC_MAX_CHARS = 120
FIELD_MAX_CHARS = 80
DECL_NAME = (
    re.compile(r"^func\s+\([^)]*\)\s*(\w+)"),
    re.compile(r"^(?:export\s+)?(?:pub\s+)?(?:async\s+)?(?:func|function|def|class|type|struct|enum|fn|const|var|let)\s+(\w+)"),
)
DOC_DECL_PATTERN = re.compile(
    r"^(?:func(?:\s*\([^)]*\))?\s+(\w+)|type\s+(\w+)|const\s+(\w+)|var\s+(\w+)|package\s+(\w+))"
)
DOC_LANGUAGE_EXTENSIONS = (".go",)
FIELD_NAME_PATTERN = re.compile(r"^\s*(\w+)\s")


def is_plain_body(body):
    if not body:
        return False
    if any(body.upper().startswith(m) for m in RESERVED_MARKERS):
        return False
    if BANNER_SHAPE.match(body) or TEMPLATE_PATTERNS["STEP"].match(body):
        return False
    return True


def validate_doc_shape(body, decl_line, is_indented):
    if is_indented or decl_line is None:
        return False, "a DOC comment must sit directly above a top-level func/type/const/var/package declaration"

    match = DOC_DECL_PATTERN.match(decl_line.strip())
    if not match:
        return False, "a DOC comment must sit directly above a func/type/const/var/package declaration"

    is_package = decl_line.strip().startswith("package ")
    name = next(g for g in match.groups() if g)
    if not is_package and not name[:1].isupper():
        return False, "a DOC comment only belongs on an exported (capitalized) declaration"

    expected_lead = f"Package {name}" if is_package else name
    if not (body == expected_lead or body.startswith(expected_lead + " ")):
        return False, f"a DOC comment must start with {expected_lead!r}"

    terminal_count = sum(body.count(c) for c in ".!?")
    if terminal_count != 1 or body[-1] not in ".!?":
        return False, "a DOC comment must be exactly one sentence"

    if len(body) > DOC_MAX_CHARS:
        return False, f"doc comment is {len(body)} chars, over the {DOC_MAX_CHARS}-char cap"

    return True, ""


def field_name_of(code_part):
    match = FIELD_NAME_PATTERN.match(code_part)
    return match.group(1) if match else None


def trailing_comment_echoes_field(code_part, comment_text):
    name = field_name_of(code_part)
    if not name:
        return False
    body = comment_body(normalize(comment_text))
    first_word = body.split()[0].strip("*(),.:;'\"`") if body else ""
    return first_word.lower() == name.lower()

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


def find_comment_start(line, family):
    line_marker, _, _, quote_chars, raw_quote_chars = FAMILY_SYNTAX[family]
    if not line_marker:
        return None
    quote_chars = quote_chars or ""
    raw_quote_chars = raw_quote_chars or ""
    i, n = 0, len(line)
    active, is_raw = None, False
    while i < n:
        ch = line[i]
        if active:
            if not is_raw and ch == "\\":
                i += 2
                continue
            if ch == active:
                active, is_raw = None, False
            i += 1
            continue
        if ch in quote_chars:
            active, is_raw = ch, False
            i += 1
            continue
        if ch in raw_quote_chars:
            active, is_raw = ch, True
            i += 1
            continue
        if line.startswith(line_marker, i):
            return i
        i += 1
    return None


def find_trailing_comments(text, family):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return []
    found = []
    for line in text.splitlines():
        idx = find_comment_start(line, family)
        if idx is None:
            continue
        if not line[:idx].strip():
            continue
        found.append(normalize(line[idx:]))
    return found


def find_doc_candidates(text, family):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return []
    lines = text.splitlines()
    candidates = []
    for index, line in enumerate(lines):
        stripped = line.strip()
        if not stripped.startswith(line_marker):
            continue
        if index > 0 and lines[index - 1].strip().startswith(line_marker):
            continue
        if index + 1 >= len(lines):
            continue
        next_line = lines[index + 1]
        if next_line.strip().startswith(line_marker):
            continue
        candidates.append((normalize(stripped), next_line))
    return candidates


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
