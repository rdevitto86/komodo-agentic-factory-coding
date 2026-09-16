"""The YAML subset task blocks use: flat keys, scalars, inline lists, dashed lists."""

from __future__ import annotations

import re
from typing import Any, Dict, List

KEY_LINE = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$")
DASH_LINE = re.compile(r"^\s*-\s+(.*)$")


class YamliteError(ValueError):
    """Raised when a block falls outside the supported subset."""


def _strip_comment(text: str) -> str:
    """Drops a trailing # comment unless the # sits inside quotes."""
    quote = None
    for index, char in enumerate(text):
        if quote:
            if char == quote:
                quote = None
            continue
        if char in "'\"":
            quote = char
            continue
        if char == "#" and (index == 0 or text[index - 1].isspace()):
            return text[:index].rstrip()
    return text.rstrip()


def _scalar(raw: str) -> Any:
    """Turns one token into str, int, bool, or None, honoring quotes."""
    text = raw.strip()
    if not text:
        return ""
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "'\"":
        return text[1:-1]
    lowered = text.lower()
    if lowered in ("true", "yes"):
        return True
    if lowered in ("false", "no"):
        return False
    if lowered in ("null", "~"):
        return None
    if re.fullmatch(r"-?\d+", text):
        return int(text)
    return text


def _split_inline_list(body: str) -> List[str]:
    """Splits [a, b, "c, d"] on commas that sit outside quotes."""
    items, current, quote = [], [], None
    for char in body:
        if quote:
            current.append(char)
            if char == quote:
                quote = None
            continue
        if char in "'\"":
            quote = char
            current.append(char)
            continue
        if char == ",":
            items.append("".join(current))
            current = []
            continue
        current.append(char)
    tail = "".join(current)
    if tail.strip() or items:
        items.append(tail)
    return [item for item in items if item.strip()]


def loads(text: str) -> Dict[str, Any]:
    """Parses a block into a dict, raising YamliteError on anything outside the subset."""
    result: Dict[str, Any] = {}
    lines = text.splitlines()
    index = 0
    while index < len(lines):
        line = _strip_comment(lines[index])
        index += 1
        if not line.strip():
            continue
        if line[0].isspace():
            raise YamliteError("unexpected indentation at: %r" % line)
        match = KEY_LINE.match(line)
        if not match:
            raise YamliteError("expected 'key: value' at: %r" % line)
        key, value = match.group(1), match.group(2).strip()
        if value.startswith("[") and value.endswith("]"):
            result[key] = [_scalar(item) for item in _split_inline_list(value[1:-1])]
            continue
        if value:
            result[key] = _scalar(value)
            continue
        items: List[Any] = []
        while index < len(lines):
            candidate = _strip_comment(lines[index])
            if not candidate.strip():
                index += 1
                continue
            dash = DASH_LINE.match(candidate)
            if not dash:
                break
            items.append(_scalar(dash.group(1)))
            index += 1
        result[key] = items
    return result


def _quote(value: Any) -> str:
    """Renders a scalar, quoting strings the parser would otherwise misread."""
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    text = str(value)
    needs_quote = (
        not text
        or text != text.strip()
        or any(char in text for char in ":#[]{},\"'")
        or text.lower() in ("true", "false", "yes", "no", "null", "~")
        or re.fullmatch(r"-?\d+", text) is not None
    )
    if needs_quote:
        return '"' + text.replace('"', '\\"') + '"'
    return text


def dumps(data: Dict[str, Any]) -> str:
    """Renders a dict back into the subset, lists dashed one per line."""
    lines = []
    for key, value in data.items():
        if isinstance(value, (list, tuple)):
            if not value:
                lines.append("%s: []" % key)
                continue
            lines.append("%s:" % key)
            lines.extend("  - %s" % _quote(item) for item in value)
            continue
        lines.append("%s: %s" % (key, _quote(value)))
    return "\n".join(lines) + ("\n" if lines else "")
