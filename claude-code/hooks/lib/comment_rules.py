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

EXTERNAL_REF_PATTERNS = (
    (re.compile(r"\bv\d+\.\d+"), "a version number"),
    (re.compile(r"\b\d+\.\d+\.\d+\b"), "a version number"),
    (re.compile(r"\b(?:PRD|SDD|ADR|TSK|EPIC|JIRA)\b"), "a spec or ticket reference"),
    (re.compile(r"(?i)\bper (?:the |our )?(?:spec|prd|sdd|design|ticket|backlog|story)"), "a document reference"),
    (re.compile(r"(?i)\bthe (?:spec|design doc|backlog|ticket|story)\b"), "a document reference"),
    (re.compile(r"(?i)\bas (?:discussed|requested|agreed)\b"), "session context"),
    (re.compile(r"(?i)\bthis (?:band|task|PR|story|sprint|session)\b"), "session context"),
    (re.compile(r"(?i)\bthe user (?:asked|wants|requested|said)\b"), "session context"),
)


# the first banned citation a comment body matches, named for the finding's message
def external_reference(body):
    for pattern, subject in EXTERNAL_REF_PATTERNS:
        if pattern.search(body):
            return subject
    return None


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
BANNER_LANGUAGE_EXTENSIONS = (".go",)
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


MANDATORY_DETAIL = {
    "RET_ARITY_3": "a discriminant return needs a comment stating what it discriminates",
    "RET_BOOL_DISCRIMINANT": "a discriminant return needs a comment stating what it discriminates",
    "FUNC_UNDOCUMENTED": "a function declaration needs a comment saying what it does",
}

KEYWORD_DECL_NAME = re.compile(
    r"^(?:(?:export|default|public|private|protected|internal|static|final|abstract"
    r"|async|inline|open|override|suspend|pub)(?:\([^)]*\))?\s+)*"
    r"(?:func|function|def|fn)\s+(\w+)"
)
ARROW_DECL_NAME = re.compile(
    r"^(?:export\s+)?(?:default\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*"
    r"(?:async\s+)?(?:\([^)]*\)|\w+)\s*(?:=>|\{)"
)
NAME_BEFORE_PAREN = re.compile(r"(\w+)\s*(?:<[^<>]*>)?\s*$")
TYPE_DECL_KEYWORDS = ("class", "record", "struct", "interface", "enum", "namespace", "trait")

TEST_NAME_PATTERNS = (
    re.compile(r"_test\.[^.]+$"),
    re.compile(r"^test_"),
    re.compile(r"\.(?:test|spec)\.[^.]+$"),
    re.compile(r"(?:Test|Tests|Spec|Specs)\.[^.]+$"),
    re.compile(r"^conftest\."),
)
TEST_PATH_PARTS = ("test", "tests", "__tests__", "spec", "specs", "testdata")
GENERATED_PATH_PARTS = ("vendor", "node_modules", "generated", "third_party")
GENERATED_SUFFIXES = (".gen.go", ".pb.go", "_pb2.py", ".g.dart", ".generated.ts", ".d.ts")
GENERATED_SCAN_LINES = 5


def path_parts(path):
    return [part.lower() for part in os.path.normpath(path).split(os.sep)[:-1]]


# whether a path names a test file or sits under a test directory
def is_test_path(path):
    if not path:
        return False
    base = os.path.basename(path)
    if any(pattern.search(base) for pattern in TEST_NAME_PATTERNS):
        return True
    return any(part in TEST_PATH_PARTS for part in path_parts(path))


# whether a file is machine-written, by its header marker, its suffix, or its directory
def is_generated(lines, path):
    for line in lines[:GENERATED_SCAN_LINES]:
        lowered = line.lower()
        if "do not edit" in lowered or "@generated" in lowered:
            return True
    if os.path.basename(path).lower().endswith(GENERATED_SUFFIXES):
        return True
    return any(part in GENERATED_PATH_PARTS for part in path_parts(path))


def function_decl_name(line, ext):
    """The name this line declares a function under, or None -- a literal, a call, and control flow all declare nothing."""
    stripped = line.strip()
    if ext in SITE_LANGUAGE_EXTENSIONS:
        parsed = parse_func_signature(stripped)
        return parsed[0] if parsed else None

    for pattern in (KEYWORD_DECL_NAME, ARROW_DECL_NAME):
        match = pattern.match(stripped)
        if match:
            return match.group(1)

    if not stripped.endswith("{"):
        return None
    groups = top_level_paren_groups(stripped)
    if not groups:
        return None
    before = stripped[: groups[0][0]]
    head = before.split()
    if not head or head[0] in NON_DECL_KEYWORDS or head[0] in TYPE_DECL_KEYWORDS:
        return None
    match = NAME_BEFORE_PAREN.search(before)
    return match.group(1) if match else None


def body_statement_count(lines, index, line_marker):
    """Statements in the declaration's body -- one or none means trivial, so no comment is demanded."""
    indent = len(lines[index]) - len(lines[index].lstrip())
    count = 0
    for line in lines[index + 1:]:
        stripped = line.strip()
        if not stripped:
            continue
        if len(line) - len(line.lstrip()) <= indent:
            break
        if line_marker and stripped.startswith(line_marker):
            continue
        count += 1
    return count


DOCSTRING_EXTENSIONS = (".py", ".pyi")
DOCSTRING_OPENERS = ('"""', "'''", 'r"""', "r'''", 'f"""', '"', "'")


def is_documented_above(lines, index, family):
    """Whether the line above carries a comment -- a line comment, or a block comment's closing line."""
    if not index:
        return False
    previous = lines[index - 1].strip()
    line_marker, _, block_close = FAMILY_SYNTAX[family][:3]
    if line_marker and previous.startswith(line_marker):
        return True
    return bool(block_close) and previous.endswith(block_close)


# whether the declaration's first body line opens a docstring
def has_docstring(lines, index):
    for line in lines[index + 1:]:
        stripped = line.strip()
        if not stripped:
            continue
        return stripped.startswith(DOCSTRING_OPENERS)
    return False


# the rule a declaration owes a comment under, most specific first, or None
def mandatory_rule(lines, index, ext, line_marker, in_test):
    line = lines[index]
    if ext in SITE_LANGUAGE_EXTENSIONS:
        parsed = parse_func_signature(line)
        if parsed:
            name, returns = parsed
            if len(returns) >= 3:
                return name, "RET_ARITY_3"
            if len(returns) >= 2 and returns[-1] == "bool" and not SITE_NAME_EXEMPT.match(name):
                return name, "RET_BOOL_DISCRIMINANT"
    if in_test or line.strip().endswith(";"):
        return None
    name = function_decl_name(line, ext)
    if not name or body_statement_count(lines, index, line_marker) <= 1:
        return None
    if ext in DOCSTRING_EXTENSIONS and has_docstring(lines, index):
        return None
    return name, "FUNC_UNDOCUMENTED"


# every declaration in the file that owes a comment and has none above it
def find_mandatory_sites(text, family, ext, path=""):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return []
    lines = text.splitlines()
    if is_generated(lines, path):
        return []
    in_test = is_test_path(path)
    sites = []
    for index in range(len(lines)):
        found = mandatory_rule(lines, index, ext, line_marker, in_test)
        if found and not is_documented_above(lines, index, family):
            sites.append((index + 1, found[0], found[1]))
    return sites


# scripts/install.py's header (23 comment lines after its shebang) is the repo's longest; 30 leaves margin.
HEADER_MAX_LINES = 30


def header_block_end(lines, line_marker):
    """Index (exclusive) of a file's leading shebang+comment header, before the first code statement or the HEADER_MAX_LINES cap, whichever comes first."""
    if not line_marker:
        return 0
    index = 1 if lines and lines[0].startswith("#!") else 0
    limit = min(len(lines), index + HEADER_MAX_LINES)
    while index < limit and lines[index].strip().startswith(line_marker):
        index += 1
    return index


FUNC_BLOCK_MAX_LINES = 2
BLOCK_MAX_LINES = 1
FUNC_KEYWORD_DECL = re.compile(
    r"^(?:(?:export|default|public|private|protected|internal|static|final|abstract"
    r"|async|inline|open|override|suspend|pub)(?:\([^)]*\))?\s+)*"
    r"(?:func|function|def|fn)\b"
)
NON_DECL_KEYWORDS = ("if", "for", "while", "switch", "catch", "else", "do", "try", "select", "case")


# whether a line opens a function body, permissively -- a literal counts, since only the cap reads this
def is_function_decl(line):
    stripped = line.strip()
    if FUNC_KEYWORD_DECL.match(stripped):
        return True
    # a keyword-less signature -- Java, C#, and C++ open a body with a parameter list and no keyword
    if not stripped.endswith("{") or not top_level_paren_groups(stripped):
        return False
    head = stripped.split("(", 1)[0].split()
    return bool(head) and head[0] not in NON_DECL_KEYWORDS


# the comment lines a declaration allows above it, and the phrase naming it in the finding
def block_line_cap(decl_line):
    if decl_line is not None and is_function_decl(decl_line):
        return FUNC_BLOCK_MAX_LINES, "above a function"
    return BLOCK_MAX_LINES, "above a var, const, type, or statement"


# every maximal run of whole-line comments, as start and exclusive-end indices
def comment_runs(lines, line_marker):
    runs, index = [], 0
    while index < len(lines):
        if not lines[index].strip().startswith(line_marker):
            index += 1
            continue
        start = index
        while index < len(lines) and lines[index].strip().startswith(line_marker):
            index += 1
        runs.append((start, index))
    return runs


# the bounds of the one comment run containing this index
def comment_run_bounds(lines, line_marker, index):
    start = index
    while start > 0 and lines[start - 1].strip().startswith(line_marker):
        start -= 1
    end = index + 1
    while end < len(lines) and lines[end].strip().startswith(line_marker):
        end += 1
    return start, end


def substantive_comment_lines(lines, start, end):
    """Indices in a comment run that count against its cap -- a directive or a bare marker does not."""
    return [index for index in range(start, end)
            if not is_mechanically_exempt(normalize(lines[index].strip()))]


# OVER_LINES for a run past its cap, STACKED for two runs too close together
def find_block_findings(lines, line_marker, only_lines):
    if not line_marker:
        return []
    header_end = header_block_end(lines, line_marker)
    findings, previous_end = [], None

    for start, end in comment_runs(lines, line_marker):
        if previous_end is not None and (start + 1) - previous_end <= ADJACENT_WINDOW:
            lineno = start + 1
            if end > header_end and (only_lines is None or lineno in only_lines):
                findings.append((lineno, comment_body(normalize(lines[start].strip())), "STACKED",
                                 "a comment already lands within %d lines, at line %d"
                                 % (ADJACENT_WINDOW, previous_end)))
        previous_end = end

        if end <= header_end:
            continue
        substantive = substantive_comment_lines(lines, start, end)
        cap, subject = block_line_cap(lines[end] if end < len(lines) else None)
        # a run that opened in the header is governed whole, so its overrun reports where the exemption ran out
        over = [index for index in substantive[cap:] if index >= header_end]
        if not over:
            continue
        if only_lines is not None and not any(index + 1 in only_lines for index in range(start, end)):
            continue
        findings.append((over[0] + 1, comment_body(normalize(lines[over[0]].strip())), "OVER_LINES",
                         "a comment block %s is capped at %d line%s; this one runs %d"
                         % (subject, cap, "" if cap == 1 else "s", len(substantive))))

    return findings


def find_invalid_comments(text, family, path, only_lines=None):
    ext = os.path.splitext(os.path.basename(path))[1].lower()
    basename = os.path.basename(path)
    line_marker = FAMILY_SYNTAX[family][0]
    lines = text.splitlines()
    header_end = header_block_end(lines, line_marker)
    findings = find_block_findings(lines, line_marker, only_lines)

    for index, line in enumerate(lines):
        lineno = index + 1
        if only_lines is not None and lineno not in only_lines:
            continue
        in_header = index < header_end

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
            if not in_header:
                findings.append((lineno, body, "STEP_MARKER", "a numbered step marker is not an allowed comment shape"))
            continue
        if BANNER_SHAPE.match(body) and not basename.endswith("_test.go") and ext in BANNER_LANGUAGE_EXTENSIONS:
            findings.append((lineno, body, "BANNER_OUTSIDE_TEST", "a banner label only belongs in a _test.go file"))
            continue
        if len(body) > cap:
            findings.append((lineno, body, "OVER_CAP", f"comment body is {len(body)} chars, over the {cap}-char cap"))
            continue
        cited = external_reference(body)
        if cited:
            findings.append((lineno, body, "EXTERNAL_REF",
                             "a comment cites %s -- describe the code, not a document, a version, or a conversation" % cited))
            continue
        for marker in ("NOTE", "FIXME", "TODO"):
            if body.upper().startswith(marker) and not TEMPLATE_PATTERNS[marker].match(body):
                findings.append((lineno, body, "MALFORMED_MARKER", f"a {marker} marker must read '{marker}: <text>'"))
                break

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
