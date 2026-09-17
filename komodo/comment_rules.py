"""Mechanical comment rules shared by the lint and the pre-commit hook."""

from __future__ import annotations

import os
import re
from typing import Dict, Iterable, List, Optional, Set, Tuple

C_FAMILY, HASH_FAMILY, DASH_FAMILY = "c", "hash", "dash"

EXTENSION_FAMILY: Dict[str, str] = {
    ".c": C_FAMILY, ".cc": C_FAMILY, ".cpp": C_FAMILY, ".cs": C_FAMILY, ".dart": C_FAMILY,
    ".go": C_FAMILY, ".h": C_FAMILY, ".hpp": C_FAMILY, ".java": C_FAMILY, ".js": C_FAMILY,
    ".jsx": C_FAMILY, ".kt": C_FAMILY, ".rs": C_FAMILY, ".swift": C_FAMILY, ".ts": C_FAMILY,
    ".tsx": C_FAMILY, ".vue": C_FAMILY, ".svelte": C_FAMILY, ".mjs": C_FAMILY, ".cjs": C_FAMILY,
    ".sh": HASH_FAMILY, ".bash": HASH_FAMILY, ".zsh": HASH_FAMILY, ".py": HASH_FAMILY, ".pyi": HASH_FAMILY,
    ".rb": HASH_FAMILY, ".yaml": HASH_FAMILY, ".yml": HASH_FAMILY, ".toml": HASH_FAMILY, ".tf": HASH_FAMILY,
    ".sql": DASH_FAMILY, ".lua": DASH_FAMILY,
}
FILENAME_FAMILY = {"Dockerfile": HASH_FAMILY, "Makefile": HASH_FAMILY, "Justfile": HASH_FAMILY}
LINE_MARKER = {C_FAMILY: "//", HASH_FAMILY: "#", DASH_FAMILY: "--"}
QUOTES = {C_FAMILY: "\"'`", HASH_FAMILY: "\"'", DASH_FAMILY: "'\""}

EXEMPT_PREFIXES = (
    "!", "+build", "-*- coding", "biome-ignore", "cgo", "clang-format", "eslint-disable", "eslint-enable",
    "fmt:", "go:", "golangci", "isort:", "istanbul ignore", "lint:", "mypy:", "nolint", "noqa", "nosec",
    "pragma", "prettier-ignore", "pylint:", "pyright:", "ruff:", "shellcheck", "ts-expect-error", "ts-ignore",
    "ts-nocheck", "type:", "use client", "use server", "@ts-", "eslint", "region", "endregion",
)

EXTERNAL_REF_PATTERNS = (
    (re.compile(r"\bv\d+\.\d+"), "a version number"),
    (re.compile(r"\b\d+\.\d+\.\d+\b"), "a version number"),
    (re.compile(r"\b(?:PRD|SDD|ADR|TSK|EPIC|JIRA|TG)-?\d*\b"), "a spec or ticket reference"),
    (re.compile(r"(?i)\bper (?:the |our )?(?:spec|prd|sdd|design|ticket|backlog|story)"), "a document reference"),
    (re.compile(r"(?i)\bthe (?:spec|design doc|backlog|ticket|story)\b"), "a document reference"),
    (re.compile(r"(?i)\bas (?:discussed|requested|agreed)\b"), "session context"),
    (re.compile(r"(?i)\bthis (?:band|task|PR|story|sprint|session)\b"), "session context"),
    (re.compile(r"(?i)\bthe user (?:asked|wants|requested|said)\b"), "session context"),
)
NARRATIVE_PATTERNS = (
    (re.compile(r"(?i)(?:^|\s)(?:we|we're|we've|i|i'm|i've|let's|our)\b"), "first person"),
    (re.compile(r"(?i)\b(?:probably|maybe|i think|should work|hopefully|seems? to|might be|kind of|sort of)\b"), "a hedge"),
    (re.compile(r"(?i)\b(?:previously|used to|no longer|now uses|was changed|refactored|moved from|instead of the old)\b"), "history"),
)
MAX_WORDS = 20
MAX_CHARS = 140
FUNC_BLOCK_MAX_LINES = 2
BLOCK_MAX_LINES = 1
HEADER_MAX_LINES = 30
MARKERS = ("NOTE", "FIXME", "TODO")
MARKER_SHAPE = re.compile(r"^(NOTE|FIXME|TODO):\s+\S")

FUNC_DECL = re.compile(
    r"^(?:(?:export|default|public|private|protected|internal|static|final|abstract|async|inline|open|override|suspend|pub)(?:\([^)]*\))?\s+)*"
    r"(?:func|function|def|fn)\s+(?:\([^)]*\)\s*)?(\w+)"
)
ARROW_DECL = re.compile(r"^(?:export\s+)?(?:default\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(?:async\s+)?(?:\([^)]*\)|\w+)\s*=>")
GO_METHOD = re.compile(r"^func\s*\([^)]*\)\s*(\w+)")
NON_DECL = ("if", "for", "while", "switch", "catch", "else", "do", "try", "select", "case", "return", "new")
TEST_NAME = (re.compile(r"_test\.[^.]+$"), re.compile(r"^test_"), re.compile(r"\.(?:test|spec)\.[^.]+$"), re.compile(r"^conftest\."))
TEST_DIRS = ("test", "tests", "__tests__", "spec", "specs", "testdata")
GENERATED_DIRS = ("vendor", "node_modules", "generated", "third_party", "dist", "build")
GENERATED_SUFFIXES = (".gen.go", ".pb.go", "_pb2.py", ".generated.ts", ".d.ts", ".min.js")
DUNDER = re.compile(r"^__\w+__$")


def resolve_family(path: str) -> Optional[str]:
    """The comment family for a path, or None when the lint has no opinion."""
    base = os.path.basename(path)
    if base in FILENAME_FAMILY:
        return FILENAME_FAMILY[base]
    return EXTENSION_FAMILY.get(os.path.splitext(base)[1].lower())


def is_test_path(path: str) -> bool:
    """Whether a path is a test file or sits under a test directory."""
    base = os.path.basename(path)
    if any(pattern.search(base) for pattern in TEST_NAME):
        return True
    parts = [part.lower() for part in os.path.normpath(path).replace("\\", "/").split("/")[:-1]]
    return any(part in TEST_DIRS for part in parts)


def is_generated(lines: List[str], path: str) -> bool:
    """Whether a file is machine-written, by header, suffix, or directory."""
    if any("do not edit" in line.lower() or "@generated" in line.lower() for line in lines[:5]):
        return True
    if os.path.basename(path).lower().endswith(GENERATED_SUFFIXES):
        return True
    parts = [part.lower() for part in os.path.normpath(path).replace("\\", "/").split("/")[:-1]]
    return any(part in GENERATED_DIRS for part in parts)


def string_masked_lines(text: str, ext: str) -> List[str]:
    """Lines with the interior of multi-line string literals blanked, so a # inside a docstring is not a comment."""
    lines = text.splitlines()
    if ext not in (".py", ".pyi"):
        return lines
    out: List[str] = []
    fence: Optional[str] = None
    for line in lines:
        if fence is not None:
            close = line.find(fence)
            if close == -1:
                out.append("")
                continue
            line = " " * (close + 3) + line[close + 3:]
            fence = None
        kept = line
        cursor = 0
        while True:
            hash_at = kept.find("#", cursor)
            positions = [(kept.find(opener, cursor), opener) for opener in ('"""', "'''") if kept.find(opener, cursor) != -1]
            if not positions:
                break
            start, opener = min(positions)
            if hash_at != -1 and hash_at < start and not _inside_simple_string(kept, hash_at):
                break
            close = kept.find(opener, start + 3)
            if close == -1:
                fence = opener
                kept = kept[:start]
                break
            kept = kept[:start] + " " * (close + 3 - start) + kept[close + 3:]
            cursor = close + 3
        out.append(kept)
    return out


def _inside_simple_string(line: str, index: int) -> bool:
    """Whether index falls inside a single-line quoted string that opened earlier on the line."""
    active = None
    for position, char in enumerate(line[:index]):
        if active:
            if char == "\\":
                continue
            if char == active:
                active = None
        elif char in "\"'":
            active = char
    return active is not None


def comment_start(line: str, family: str) -> Optional[int]:
    """Index where a comment begins on this line, ignoring markers inside quotes, or None."""
    marker = LINE_MARKER[family]
    quotes = QUOTES[family]
    active = None
    index = 0
    while index < len(line):
        char = line[index]
        if active:
            if char == "\\":
                index += 2
                continue
            if char == active:
                active = None
        elif char in quotes:
            active = char
        elif line.startswith(marker, index):
            return index
        index += 1
    return None


def comment_body(raw: str, family: str) -> str:
    """The text of a comment with its marker and any leading doc punctuation removed."""
    text = raw.strip()
    marker = LINE_MARKER[family]
    while text.startswith(marker):
        text = text[len(marker):]
    return text.strip().lstrip("/!*").strip()


def is_directive(body: str) -> bool:
    """Whether a comment body is a machine directive the lint leaves alone."""
    lowered = body.lower()
    return not body or any(lowered.startswith(prefix) for prefix in EXEMPT_PREFIXES)


def external_reference(body: str) -> Optional[str]:
    """The first banned citation a body contains, named for the message."""
    for pattern, subject in EXTERNAL_REF_PATTERNS:
        if pattern.search(body):
            return subject
    return None


def narrative(body: str) -> Optional[str]:
    """The first narrative tell a body contains, named for the message."""
    for pattern, subject in NARRATIVE_PATTERNS:
        if pattern.search(body):
            return subject
    return None


def function_name(line: str, ext: str) -> Optional[str]:
    """The function a line declares, or None for a call, a literal, or control flow."""
    stripped = line.strip()
    if ext == ".go":
        match = GO_METHOD.match(stripped) or re.match(r"^func\s+(\w+)", stripped)
        return match.group(1) if match else None
    for pattern in (FUNC_DECL, ARROW_DECL):
        match = pattern.match(stripped)
        if match:
            return match.group(1)
    if ext in (".java", ".cs", ".kt", ".c", ".cc", ".cpp", ".h", ".hpp", ".swift", ".rs", ".dart") and stripped.endswith("{") and "(" in stripped:
        head = stripped.split("(", 1)[0].split()
        if head and head[0] not in NON_DECL and not head[0].startswith(("class", "struct", "enum", "interface", "record")):
            return head[-1]
    return None


def is_exported(name: str, line: str, ext: str) -> bool:
    """Whether a declaration is part of the public surface under its language's convention."""
    stripped = line.strip()
    if ext == ".go":
        return name[:1].isupper()
    if ext in (".py", ".pyi"):
        return not name.startswith("_")
    if ext in (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte"):
        return stripped.startswith("export")
    if ext == ".rs":
        return stripped.startswith("pub")
    return "public" in stripped.split("(")[0] or "protected" in stripped.split("(")[0]


def body_span(lines: List[str], index: int, family: str) -> Tuple[int, int]:
    """Statement lines and return count inside a declaration's body, read from indentation."""
    indent = len(lines[index]) - len(lines[index].lstrip())
    marker = LINE_MARKER[family]
    statements = 0
    returns = 0
    for line in lines[index + 1:]:
        stripped = line.strip()
        if not stripped:
            continue
        if len(line) - len(line.lstrip()) <= indent and not stripped.startswith(("}", ")", "]")):
            break
        if stripped.startswith(marker) or stripped in ("}", "})", "};"):
            continue
        statements += 1
        if stripped.startswith("return"):
            returns += 1
    return statements, returns


def documented_above(lines: List[str], index: int, family: str) -> bool:
    """Whether the previous non-blank line is a comment or closes a block comment."""
    marker = LINE_MARKER[family]
    look = index - 1
    while look >= 0 and not lines[look].strip():
        look -= 1
    if look < 0:
        return False
    previous = lines[look].strip()
    return previous.startswith(marker) or previous.endswith("*/") or previous.startswith(("/**", "/*", "///"))


def has_docstring(lines: List[str], index: int) -> bool:
    """Whether the first body line opens a Python docstring."""
    for line in lines[index + 1:]:
        stripped = line.strip()
        if stripped:
            return stripped.startswith(('"""', "'''", 'r"""', "r'''"))
    return False


def undocumented_functions(text: str, path: str, require: str = "nonobvious", trivial_lines: int = 8) -> List[Tuple[int, str]]:
    """Function declarations owing a comment under the configured requirement and lacking one."""
    if require == "none":
        return []
    family = resolve_family(path)
    if family is None or is_test_path(path):
        return []
    ext = os.path.splitext(path)[1].lower()
    raw_lines = text.splitlines()
    if is_generated(raw_lines, path):
        return []
    lines = string_masked_lines(text, ext)
    found: List[Tuple[int, str]] = []
    for index, line in enumerate(lines):
        name = function_name(line, ext)
        if not name or DUNDER.match(name):
            continue
        statements, returns = body_span(lines, index, family)
        if statements == 0:
            continue
        exported = is_exported(name, line, ext)
        nonobvious = statements > trivial_lines or returns > 1
        if require == "exported" and not exported:
            continue
        if require == "nonobvious" and not (exported or nonobvious):
            continue
        if ext in (".py", ".pyi") and has_docstring(raw_lines, index):
            continue
        if documented_above(raw_lines, index, family):
            continue
        found.append((index + 1, name))
    return found


def _is_function_line(line: str, ext: str) -> bool:
    """Whether a line opens a function, permissively."""
    return function_name(line, ext) is not None or bool(FUNC_DECL.match(line.strip()))


def invalid_comments(text: str, path: str, only_lines: Optional[Set[int]] = None) -> List[Tuple[int, str, str]]:
    """Every comment breaking a mechanical rule: (line, rule, detail)."""
    family = resolve_family(path)
    if family is None:
        return []
    ext = os.path.splitext(path)[1].lower()
    marker = LINE_MARKER[family]
    lines = string_masked_lines(text, ext)
    findings: List[Tuple[int, str, str]] = []
    header_end = 0
    index = 1 if lines and lines[0].startswith("#!") else 0
    while index < min(len(lines), HEADER_MAX_LINES) and lines[index].strip().startswith(marker):
        index += 1
    header_end = index

    runs: List[Tuple[int, int]] = []
    index = 0
    while index < len(lines):
        if lines[index].strip().startswith(marker):
            start = index
            while index < len(lines) and lines[index].strip().startswith(marker):
                index += 1
            runs.append((start, index))
        else:
            index += 1

    previous_end = None
    for start, end in runs:
        in_scope = only_lines is None or any(number + 1 in only_lines for number in range(start, end))
        only_blank_between = previous_end is not None and all(not lines[number].strip() for number in range(previous_end, start))
        if only_blank_between and end > header_end and in_scope:
            findings.append((start + 1, "STACKED", "two comment blocks with no code between them; merge or drop one"))
        previous_end = end
        if end <= header_end or not in_scope:
            continue
        substantive = [number for number in range(start, end) if not is_directive(comment_body(lines[number], family))]
        target = lines[end] if end < len(lines) else ""
        cap = FUNC_BLOCK_MAX_LINES if _is_function_line(target, ext) else BLOCK_MAX_LINES
        if len(substantive) > cap:
            findings.append((substantive[cap] + 1, "OVER_LINES", "a comment block %s is capped at %d line%s; this one runs %d" % ("above a function" if cap == 2 else "above a statement", cap, "" if cap == 1 else "s", len(substantive))))

    for number, line in enumerate(lines):
        lineno = number + 1
        if only_lines is not None and lineno not in only_lines:
            continue
        if number < header_end:
            continue
        start = comment_start(line, family)
        if start is None:
            continue
        body = comment_body(line[start:], family)
        if is_directive(body):
            continue
        words = len(body.split())
        if len(body) > MAX_CHARS or words > MAX_WORDS:
            findings.append((lineno, "OVER_WORDS", "comment runs %d words; the cap is %d" % (words, MAX_WORDS)))
            continue
        cited = external_reference(body)
        if cited:
            findings.append((lineno, "EXTERNAL_REF", "comment cites %s; describe the code, not a document, version, or conversation" % cited))
            continue
        tell = narrative(body)
        if tell:
            findings.append((lineno, "NARRATIVE", "comment carries %s; state what the code does" % tell))
            continue
        upper = body.upper()
        if upper.startswith(MARKERS) and not MARKER_SHAPE.match(body):
            findings.append((lineno, "MALFORMED_MARKER", "a marker must read 'NOTE: text', 'TODO: text', or 'FIXME: text'"))
            continue
        if line.strip().startswith(marker) and number + 1 < len(lines):
            name = function_name(lines[number + 1], ext)
            first = body.split()[0].strip("*(),.:;'\"`").lower() if body else ""
            if name and first == name.lower() and ext != ".go" and words <= 3:
                findings.append((lineno, "NAME_ECHO", "comment restates the identifier below it"))
    return sorted(set(findings))
