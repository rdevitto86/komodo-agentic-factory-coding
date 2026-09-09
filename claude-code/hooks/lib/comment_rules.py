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

SITE_LANGUAGE_EXTENSIONS = (".go",)
SITE_NAME_EXEMPT = re.compile(r"^(?:[Ii]s|[Hh]as|[Cc]an|[Ss]hould|[Ee]xists|[Mm]ust)(?:[A-Z]|$)")
ADJACENT_WINDOW = 2


def top_level_paren_groups(line):
    groups, depth, start = [], 0, None
    for index, char in enumerate(line):
        if char == "(":
            if depth == 0:
                start = index
            depth += 1
        elif char == ")" and depth > 0:
            depth -= 1
            if depth == 0 and start is not None:
                groups.append((start, index, line[start + 1:index]))
    return groups


def split_top_level(text):
    parts, depth, current = [], 0, []
    for char in text:
        if char in "([{":
            depth += 1
        elif char in ")]}":
            depth -= 1
        if char == "," and depth == 0:
            parts.append("".join(current).strip())
            current = []
            continue
        current.append(char)
    tail = "".join(current).strip()
    if tail:
        parts.append(tail)
    return [part for part in parts if part]


def parse_func_signature(line):
    stripped = line.strip()
    if not stripped.startswith("func"):
        return None
    groups = top_level_paren_groups(stripped)
    if not groups:
        return None

    after_keyword = stripped[4:].lstrip()
    has_receiver = after_keyword.startswith("(")
    params_index = 1 if has_receiver else 0
    if len(groups) <= params_index:
        return None

    name_start = groups[params_index - 1][1] + 1 if has_receiver else 4
    name = stripped[name_start:groups[params_index][0]].strip()
    if not name or not name.replace("_", "").isalnum():
        return None

    if len(groups) <= params_index + 1:
        return name, []
    return name, [
        part.split()[-1] for part in split_top_level(groups[params_index + 1][2]) if part.split()
    ]


def find_mandatory_sites(text, family, ext):
    if ext not in SITE_LANGUAGE_EXTENSIONS:
        return []
    line_marker = FAMILY_SYNTAX[family][0]
    lines = text.splitlines()
    sites = []
    for index, line in enumerate(lines):
        parsed = parse_func_signature(line)
        if not parsed:
            continue
        name, returns = parsed
        if len(returns) >= 3:
            rule = "RET_ARITY_3"
        elif len(returns) >= 2 and returns[-1] == "bool" and not SITE_NAME_EXEMPT.match(name):
            rule = "RET_BOOL_DISCRIMINANT"
        else:
            continue
        previous = lines[index - 1].strip() if index else ""
        if line_marker and previous.startswith(line_marker):
            continue
        sites.append((index + 1, name, rule))
    return sites


def find_invalid_comments(text, family, path, only_lines=None):
    ext = os.path.splitext(os.path.basename(path))[1].lower()
    basename = os.path.basename(path)
    line_marker = FAMILY_SYNTAX[family][0]
    lines = text.splitlines()
    findings = []
    seen_comment_lines = []

    for index, line in enumerate(lines):
        lineno = index + 1
        if only_lines is not None and lineno not in only_lines:
            continue

        start = find_comment_start(line, family) if line_marker else None
        stripped = line.strip()
        is_leading = bool(line_marker) and stripped.startswith(line_marker)
        if start is None and not is_leading:
            continue

        normalized = normalize(stripped if is_leading else line[start:])
        if is_mechanically_exempt(normalized):
            continue
        body = comment_body(normalized)
        if not body:
            continue

        cap = FIELD_MAX_CHARS if not is_leading else NARRATIVE_MAX_CHARS
        if TEMPLATE_PATTERNS["STEP"].match(body):
            findings.append((lineno, body, "STEP_MARKER", "a numbered step marker is not an allowed comment shape"))
            continue
        if BANNER_SHAPE.match(body) and not basename.endswith("_test.go"):
            findings.append((lineno, body, "BANNER_OUTSIDE_TEST", "a banner label only belongs in a _test.go file"))
            continue
        if len(body) > cap:
            findings.append((lineno, body, "OVER_CAP", f"comment body is {len(body)} chars, over the {cap}-char cap"))
            continue
        for marker in ("NOTE", "FIXME", "TODO"):
            if body.upper().startswith(marker) and not TEMPLATE_PATTERNS[marker].match(body):
                findings.append((lineno, body, "MALFORMED_MARKER", f"a {marker} marker must read '{marker}: <text>'"))
                break
        else:
            if is_leading:
                near = next((ln for ln in seen_comment_lines if abs(ln - lineno) <= ADJACENT_WINDOW), None)
                if near is not None:
                    findings.append((lineno, body, "STACKED", f"a comment already lands within {ADJACENT_WINDOW} lines, at line {near}"))
                seen_comment_lines.append(lineno)

    echoes = check_echoes(lines, family)
    for index, line in enumerate(lines):
        lineno = index + 1
        if only_lines is not None and lineno not in only_lines:
            continue
        if line.strip() not in echoes:
            continue
        decl_line = lines[index + 1] if index + 1 < len(lines) else None
        ok, _ = validate_doc_shape(comment_body(normalize(line.strip())), decl_line, False)
        if ok and ext in DOC_LANGUAGE_EXTENSIONS:
            continue
        findings.append((lineno, comment_body(normalize(line.strip())), "NAME_ECHO", "the comment restates the name of the declaration below it"))

    return sorted(findings)


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
