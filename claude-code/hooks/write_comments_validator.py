#!/usr/bin/env python3
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from lib.comment_rules import (
    FAMILY_SYNTAX,
    TEMPLATE_PATTERNS,
    check_echoes,
    comment_body,
    is_narrative_template,
    normalize,
    resolve_family,
    scan_comments,
)

TEMPLATE_TYPE_PATTERN_INDEX = {
    "BANNER": 0,
    "WHY": 1,
    "NOTE": 1,
    "FIXME": 1,
    "HACK": 1,
    "TODO": 2,
    "STEP": 3,
}


def classify_template_index(body):
    for idx, pattern in enumerate(TEMPLATE_PATTERNS):
        if pattern.match(body):
            return idx
    return None


def validate_comment_shape(text, template_type, family, is_indented):
    line_marker = FAMILY_SYNTAX[family][0]
    if not line_marker:
        return False, f"the {family!r} family has no line-comment marker"

    normalized = normalize(text)
    if not normalized.startswith(line_marker):
        return False, f"text does not start with the {family!r} family's line marker {line_marker!r}"

    body = comment_body(normalized)
    if not body:
        return False, "comment body is empty"

    expected_idx = TEMPLATE_TYPE_PATTERN_INDEX.get(template_type)
    if expected_idx is None:
        return False, f"unknown template_type {template_type!r}"

    matched_idx = classify_template_index(body)

    if template_type == "STEP":
        if matched_idx is not None and matched_idx != expected_idx:
            return False, "text matches a different template shape than the claimed STEP"
    elif matched_idx != expected_idx:
        return False, f"text does not match the {template_type} template shape"

    if not is_narrative_template(normalized, is_indented=is_indented):
        return False, f"text does not match the {template_type} template shape"

    return True, ""


def resolve_proposal_path(file_field, repo_root):
    if os.path.isabs(file_field):
        return file_field
    return os.path.join(repo_root, file_field)


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
    if not template_type or template_type not in TEMPLATE_TYPE_PATTERN_INDEX:
        return None, None, None, f"missing or unknown template_type {template_type!r}"
    if not text or not isinstance(text, str):
        return None, None, None, "missing or invalid 'text'"

    full_path = resolve_proposal_path(file_field, repo_root)
    family = resolve_family(full_path)
    if not family:
        return None, None, None, f"unresolvable file family for {file_field!r}"
    if not os.path.isfile(full_path):
        return None, None, None, f"file does not exist: {file_field!r}"

    ext = os.path.splitext(os.path.basename(full_path))[1].lower()
    return full_path, family, ext, None


def apply_proposal_to_lines(proposal, family, lines, old_comment_set):
    line_no = proposal["line"]
    template_type = proposal["template_type"]
    text = proposal["text"]

    if line_no < 1 or line_no > len(lines) + 1:
        return None, f"line {line_no} is out of range ({len(lines)} lines)"

    ref_line = lines[line_no - 1] if line_no - 1 < len(lines) else (lines[-1] if lines else "")
    indent = ref_line[: len(ref_line) - len(ref_line.lstrip())]
    is_indented = len(indent) > 0

    ok, reason = validate_comment_shape(text, template_type, family, is_indented)
    if not ok:
        return None, reason

    indented_text = indent + normalize(text)

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
        for idx, proposal in items:
            result, reason = apply_proposal_to_lines(proposal, family, lines, old_comment_set)
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
