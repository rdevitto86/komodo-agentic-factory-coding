#!/usr/bin/env python3
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from auto_format import run_formatter
from lib.comment_rules import (
    DOC_LANGUAGE_EXTENSIONS,
    FAMILY_SYNTAX,
    FIELD_MAX_CHARS,
    NARRATIVE_MAX_CHARS,
    STEP_MAX_CHARS,
    TEMPLATE_PATTERNS,
    BANNER_LABEL,
    check_echoes,
    comment_body,
    find_comment_start,
    is_plain_body,
    normalize,
    resolve_family,
    scan_comments,
    trailing_comment_echoes_field,
    validate_doc_shape,
)

ADJACENT_WINDOW = 2
KNOWN_TEMPLATE_TYPES = ("BANNER", "WHY", "HACK", "NOTE", "FIXME", "TODO", "STEP", "DOC", "FIELD")


def validate_comment_shape(text, template_type, family, is_indented, decl_line=None):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return False, f"the {family!r} family has no line-comment marker"

    normalized = normalize(text)
    if not normalized.startswith(line_marker):
        return False, f"text does not start with the {family!r} family's line marker {line_marker!r}"

    body = comment_body(normalized)
    if not body:
        return False, "comment body is empty"

    if template_type not in KNOWN_TEMPLATE_TYPES:
        return False, f"unknown template_type {template_type!r}"

    if template_type in ("NOTE", "FIXME", "TODO"):
        if not TEMPLATE_PATTERNS[template_type].match(body):
            return False, f"text does not match the {template_type} template shape"
        if len(body) > NARRATIVE_MAX_CHARS:
            return False, f"comment body is {len(body)} chars, over the {NARRATIVE_MAX_CHARS}-char cap -- state one fact, not several"
        return True, ""

    if template_type == "BANNER":
        if not TEMPLATE_PATTERNS["BANNER"].match(body):
            return False, f"a banner label must be exactly {BANNER_LABEL!r}"
        return True, ""

    if template_type == "STEP":
        if not TEMPLATE_PATTERNS["STEP"].match(body):
            return False, "text does not match the STEP template shape"
        if not is_indented:
            return False, "a STEP marker only belongs inside an indented function body"
        if len(body) > STEP_MAX_CHARS:
            return False, f"step body is {len(body)} chars, over the {STEP_MAX_CHARS}-char cap"
        return True, ""

    if template_type in ("WHY", "HACK"):
        if not is_plain_body(body):
            return False, "text must be a plain sentence with no marker prefix"
        if len(body) > NARRATIVE_MAX_CHARS:
            return False, f"comment body is {len(body)} chars, over the {NARRATIVE_MAX_CHARS}-char cap -- state one fact, not several"
        return True, ""

    if template_type == "FIELD":
        if not is_plain_body(body):
            return False, "text must be a plain clause with no marker prefix"
        if len(body) > FIELD_MAX_CHARS:
            return False, f"field comment is {len(body)} chars, over the {FIELD_MAX_CHARS}-char cap"
        return True, ""

    return validate_doc_shape(body, decl_line, is_indented)


def resolve_proposal_path(file_field, repo_root):
    if os.path.isabs(file_field):
        return file_field
    return os.path.join(repo_root, file_field)


def is_within_repo_root(full_path, repo_root):
    real_path = os.path.realpath(full_path)
    real_root = os.path.realpath(repo_root)
    if real_path == real_root:
        return True
    return real_path.startswith(real_root + os.sep)


def prevalidate_proposal(proposal, repo_root):
    if not isinstance(proposal, dict):
        return None, None, None, "proposal is not a JSON object"

    file_field = proposal.get("file")
    line_no = proposal.get("line")
    template_type = proposal.get("template_type")
    text = proposal.get("text")

    if not file_field or not isinstance(file_field, str):
        return None, None, None, "missing or invalid 'file'"
    if not isinstance(line_no, int) or isinstance(line_no, bool):
        return None, None, None, "missing or invalid 'line'"
    if not template_type or template_type not in KNOWN_TEMPLATE_TYPES:
        return None, None, None, f"missing or unknown template_type {template_type!r}"
    if not text or not isinstance(text, str):
        return None, None, None, "missing or invalid 'text'"

    full_path = resolve_proposal_path(file_field, repo_root)
    if not is_within_repo_root(full_path, repo_root):
        return None, None, None, f"resolved path is outside repo root: {file_field!r}"

    if template_type == "BANNER" and not os.path.basename(full_path).endswith("_test.go"):
        return None, None, None, "a BANNER comment is only allowed in a _test.go file"

    if template_type == "DOC" and not full_path.endswith(DOC_LANGUAGE_EXTENSIONS):
        return None, None, None, f"a DOC comment is only allowed in {DOC_LANGUAGE_EXTENSIONS!r} files"

    family = resolve_family(full_path)
    if not family:
        return None, None, None, f"unresolvable file family for {file_field!r}"
    if not os.path.isfile(full_path):
        return None, None, None, f"file does not exist: {file_field!r}"

    ext = os.path.splitext(os.path.basename(full_path))[1].lower()
    return full_path, family, ext, None


def apply_field_proposal(proposal, family, lines):
    line_no = proposal["line"]
    text = proposal["text"]

    if line_no < 1 or line_no > len(lines):
        return None, f"line {line_no} is out of range ({len(lines)} lines) -- a FIELD comment appends to an existing line"

    target = lines[line_no - 1]
    if not target.strip():
        return None, "a FIELD comment must attach to a non-blank line"

    if find_comment_start(target, family) is not None:
        return None, "this line already carries a trailing comment"

    ok, reason = validate_comment_shape(text, "FIELD", family, is_indented=False)
    if not ok:
        return None, reason

    if trailing_comment_echoes_field(target, text):
        return None, "this comment would echo the field name it trails"

    lines[line_no - 1] = target.rstrip() + "  " + normalize(text)
    return lines[line_no - 1], None


def apply_proposal_to_lines(proposal, family, lines, old_comment_set, accepted_lines=()):
    line_no = proposal["line"]
    template_type = proposal["template_type"]
    text = proposal["text"]

    if template_type == "FIELD":
        return apply_field_proposal(proposal, family, lines)

    if line_no < 1 or line_no > len(lines) + 1:
        return None, f"line {line_no} is out of range ({len(lines)} lines)"

    ref_line = lines[line_no - 1] if line_no - 1 < len(lines) else (lines[-1] if lines else "")
    indent = ref_line[: len(ref_line) - len(ref_line.lstrip())]
    is_indented = len(indent) > 0

    ok, reason = validate_comment_shape(text, template_type, family, is_indented, decl_line=ref_line)
    if not ok:
        return None, reason

    near = next((ln for ln in accepted_lines if abs(ln - line_no) <= ADJACENT_WINDOW), None)
    if near is not None:
        return None, f"a comment already lands within {ADJACENT_WINDOW} lines of this one, at line {near} -- one comment per site, not a stack"

    indented_text = indent + normalize(text)

    if template_type != "DOC":
        trial = lines[: line_no - 1] + [indented_text] + lines[line_no - 1 :]
        echoes = check_echoes(trial, family, old_comment_set)
        if indented_text.strip() in echoes:
            return None, "inserting this comment would echo the identifier on the following line"

    lines[line_no - 1 : line_no - 1] = [indented_text]
    return indented_text, None


def process_proposals(proposals, repo_root):
    spliced = [None] * len(proposals)
    dropped = [None] * len(proposals)

    by_file = {}
    for idx, proposal in enumerate(proposals):
        full_path, family, ext, error = prevalidate_proposal(proposal, repo_root)
        if error:
            dropped[idx] = {
                "file": proposal.get("file") if isinstance(proposal, dict) else None,
                "line": proposal.get("line") if isinstance(proposal, dict) else None,
                "text": proposal.get("text") if isinstance(proposal, dict) else None,
                "reason": error,
            }
            continue
        by_file.setdefault(full_path, {"family": family, "ext": ext, "items": []})
        by_file[full_path]["items"].append((idx, proposal))

    for full_path, group in by_file.items():
        family, ext, items = group["family"], group["ext"], group["items"]
        with open(full_path, "r", encoding="utf-8", errors="ignore") as f:
            old_text = f.read()
        had_trailing_newline = old_text.endswith("\n")
        lines = old_text.splitlines()
        old_comment_set = set(c[0] for c in scan_comments(old_text, family, ext))

        items.sort(key=lambda pair: pair[1]["line"], reverse=True)

        touched = False
        accepted_lines = []
        for idx, proposal in items:
            result, reason = apply_proposal_to_lines(proposal, family, lines, old_comment_set, accepted_lines)
            if not reason and proposal["template_type"] != "FIELD":
                accepted_lines.append(proposal["line"])
            if reason:
                dropped[idx] = {
                    "file": proposal["file"],
                    "line": proposal["line"],
                    "text": proposal["text"],
                    "reason": reason,
                }
            else:
                touched = True
                spliced[idx] = {
                    "file": proposal["file"],
                    "line": proposal["line"],
                    "text": result,
                }

        if touched:
            new_text = "\n".join(lines)
            if had_trailing_newline:
                new_text += "\n"
            with open(full_path, "w", encoding="utf-8") as f:
                f.write(new_text)
            run_formatter(full_path)

    return [s for s in spliced if s is not None], [d for d in dropped if d is not None]


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Deterministic validator/splice gate between write-comments proposals "
            "and the repo's files. Reads a JSON array of proposals from stdin, "
            "each shaped {\"file\": <repo-relative or absolute path>, \"line\": "
            "<int>, \"template_type\": one of WHY/NOTE/FIXME/HACK/TODO/BANNER/STEP, "
            "\"text\": \"<the exact, already-marker-formatted comment line\"}. "
            "Prints {\"spliced\": [...], \"dropped\": [...]} to stdout."
        ),
    )
    parser.add_argument(
        "--repo-root",
        default=os.getcwd(),
        help=(
            "Root a proposal's 'file' path is resolved against when it is not "
            "already absolute. Defaults to the current working directory."
        ),
    )
    parser.add_argument(
        "--line-convention",
        action="store_true",
        help=(
            "No-op flag documenting the line-number convention: 'line' is "
            "1-indexed and names the position the new comment line takes in "
            "the resulting file -- the line currently at that number (and "
            "everything after it) shifts down by one. A value equal to "
            "len(file)+1 appends after the last line."
        ),
    )
    args = parser.parse_args()

    raw = sys.stdin.read()
    try:
        proposals = json.loads(raw) if raw.strip() else []
    except json.JSONDecodeError as exc:
        print(f"could not parse proposals JSON: {exc}", file=sys.stderr)
        json.dump({"spliced": [], "dropped": []}, sys.stdout)
        return

    if not isinstance(proposals, list):
        print("proposals payload must be a JSON array", file=sys.stderr)
        json.dump({"spliced": [], "dropped": []}, sys.stdout)
        return

    spliced, dropped = process_proposals(proposals, args.repo_root)

    for entry in dropped:
        print(f"dropped {entry['file']!r}:{entry['line']} -- {entry['reason']}", file=sys.stderr)

    json.dump({"spliced": spliced, "dropped": dropped}, sys.stdout)


if __name__ == "__main__":
    main()
