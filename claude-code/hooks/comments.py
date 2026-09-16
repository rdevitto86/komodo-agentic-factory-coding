#!/usr/bin/env python3
import argparse
import json
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from auto_format import run_formatter
from lib.comment_rules import (
    ADJACENT_WINDOW,
    DOC_LANGUAGE_EXTENSIONS,
    FAMILY_SYNTAX,
    FIELD_MAX_CHARS,
    MANDATORY_DETAIL,
    NARRATIVE_MAX_CHARS,
    STEP_MAX_CHARS,
    TEMPLATE_PATTERNS,
    BANNER_LABEL,
    block_line_cap,
    check_echoes,
    comment_body,
    comment_run_bounds,
    external_reference,
    find_comment_start,
    header_block_end,
    substantive_comment_lines,
    find_invalid_comments,
    find_mandatory_sites,
    is_plain_body,
    normalize,
    resolve_family,
    scan_comments,
    trailing_comment_echoes_field,
    validate_doc_shape,
)

KNOWN_TEMPLATE_TYPES = ("BANNER", "WHY", "HACK", "NOTE", "FIXME", "TODO", "STEP", "DOC", "FIELD")

# a hand-edited source file this large is not a realistic target -- treat it as a reason to skip, not read
HOOK_MAX_FILE_BYTES = 1_000_000


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

    cited = external_reference(body)
    if cited:
        return False, f"text cites {cited} -- describe the code, not a document, a version, or a conversation"

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
    trial = lines[: line_no - 1] + [indented_text] + lines[line_no - 1 :]

    if template_type != "DOC":
        echoes = check_echoes(trial, family, old_comment_set)
        if indented_text.strip() in echoes:
            return None, "inserting this comment would echo the identifier on the following line"

    line_marker = FAMILY_SYNTAX[family][0]
    start, end = comment_run_bounds(trial, line_marker, line_no - 1)
    if end > header_block_end(trial, line_marker):
        cap, subject = block_line_cap(trial[end] if end < len(trial) else None)
        count = len(substantive_comment_lines(trial, start, end))
        if count > cap:
            return None, ("this would make a %d-line comment block %s, over the %d-line cap"
                          % (count, subject, cap))

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


def untracked_files(repo_root):
    try:
        result = subprocess.run(
            ["git", "ls-files", "--others", "--exclude-standard"],
            cwd=repo_root, capture_output=True, text=True, timeout=30,
        )
    except (OSError, subprocess.SubprocessError):
        return []
    if result.returncode != 0:
        return []
    return [line for line in result.stdout.splitlines() if line]


def changed_line_map(base, repo_root):
    try:
        result = subprocess.run(
            ["git", "diff", "--unified=0", base, "--"],
            cwd=repo_root, capture_output=True, text=True, timeout=30,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None

    changed, path, lineno = {}, None, 0
    for line in result.stdout.splitlines():
        if line.startswith("+++ b/"):
            path = line[6:]
            changed.setdefault(path, set())
        elif line.startswith("@@"):
            marker = line.split("+", 1)
            if len(marker) > 1:
                span = marker[1].split("@@")[0].strip().split(",")
                try:
                    lineno = int(span[0])
                except ValueError:
                    lineno = 0
        elif line.startswith("+") and not line.startswith("+++") and path is not None:
            changed[path].add(lineno)
            lineno += 1

    # a new file has no diff hunks until staged, so fold it in as wholly changed instead of switching to --all
    for path in untracked_files(repo_root):
        full_path = os.path.join(repo_root, path)
        try:
            with open(full_path, "r", encoding="utf-8", errors="ignore") as handle:
                total_lines = len(handle.readlines())
        except OSError:
            continue
        changed[path] = set(range(1, total_lines + 1))
    return changed


def collect_files(paths, repo_root):
    found = []
    for target in paths:
        full = target if os.path.isabs(target) else os.path.join(repo_root, target)
        if os.path.isfile(full):
            found.append(full)
            continue
        for root, dirs, names in os.walk(full):
            dirs[:] = [d for d in dirs if d not in (".git", "node_modules", "vendor", ".venv")]
            found.extend(os.path.join(root, name) for name in names)
    return [f for f in found if resolve_family(f)]


def collect_findings(full_path, relative, family, ext, text, only_lines):
    findings = []

    for lineno, name, rule in find_mandatory_sites(text, family, ext, relative):
        if only_lines is not None and lineno not in only_lines:
            continue
        findings.append({
            "file": relative, "line": lineno, "kind": "MISSING",
            "rule": rule, "subject": name, "detail": MANDATORY_DETAIL[rule],
        })

    for lineno, body, rule, detail in find_invalid_comments(text, family, full_path, only_lines):
        findings.append({
            "file": relative, "line": lineno, "kind": "INVALID",
            "rule": rule, "subject": body, "detail": detail,
        })

    return findings


def run_check(args):
    repo_root = os.path.abspath(args.repo_root)
    changed = None if args.all else changed_line_map(args.base, repo_root)
    findings = []

    for full_path in collect_files(args.paths or [repo_root], repo_root):
        relative = os.path.relpath(full_path, repo_root)
        only_lines = None
        if changed is not None:
            only_lines = changed.get(relative)
            if not only_lines:
                continue

        family = resolve_family(full_path)
        ext = os.path.splitext(os.path.basename(full_path))[1].lower()
        try:
            with open(full_path, "r", encoding="utf-8", errors="ignore") as handle:
                text = handle.read()
        except OSError:
            continue

        findings.extend(collect_findings(full_path, relative, family, ext, text, only_lines))

    findings.sort(key=lambda f: (f["file"], f["line"]))
    if args.json:
        json.dump({"findings": findings}, sys.stdout, indent=2)
        sys.stdout.write("\n")
    else:
        for finding in findings:
            print("{file}:{line}: {kind} {rule} -- {detail}".format(**finding))
        print("%d finding(s)" % len(findings))
    return 1 if findings else 0


def run_apply(args):
    raw = sys.stdin.read()
    try:
        proposals = json.loads(raw) if raw.strip() else []
    except json.JSONDecodeError as exc:
        print("could not parse proposals JSON: %s" % exc, file=sys.stderr)
        json.dump({"spliced": [], "dropped": []}, sys.stdout)
        return 1

    if not isinstance(proposals, list):
        print("proposals payload must be a JSON array", file=sys.stderr)
        json.dump({"spliced": [], "dropped": []}, sys.stdout)
        return 1

    spliced, dropped = process_proposals(proposals, args.repo_root)
    for entry in dropped:
        print("dropped %r:%s -- %s" % (entry["file"], entry["line"], entry["reason"]), file=sys.stderr)
    json.dump({"spliced": spliced, "dropped": dropped}, sys.stdout)
    sys.stdout.write("\n")
    return 0


def hook_findings(full_path, root):
    if not is_within_repo_root(full_path, root):
        return None

    family = resolve_family(full_path)
    if not family:
        return None

    try:
        if os.path.getsize(full_path) > HOOK_MAX_FILE_BYTES:
            return None
    except OSError:
        return None

    changed = changed_line_map("HEAD", root)
    relative = os.path.relpath(full_path, root)
    only_lines = None
    if changed is not None:
        only_lines = changed.get(relative)
        if not only_lines:
            return []

    ext = os.path.splitext(os.path.basename(full_path))[1].lower()
    with open(full_path, "r", encoding="utf-8", errors="ignore") as handle:
        text = handle.read(HOOK_MAX_FILE_BYTES)

    findings = collect_findings(full_path, relative, family, ext, text, only_lines)
    findings.sort(key=lambda f: f["line"])
    return findings


def run_hook(args):
    # feedback, not a gate -- every path below returns 0, so a crash here can never block the write
    try:
        payload = json.loads(sys.stdin.read())
        if not isinstance(payload, dict):
            return 0

        tool_input = payload.get("tool_input")
        file_path = tool_input.get("file_path") if isinstance(tool_input, dict) else None
        if not file_path or not os.path.isfile(file_path):
            return 0

        # realpath, not abspath, and rooted at repo_root rather than the target's own git ancestry
        full_path = os.path.realpath(file_path)
        root = os.path.realpath(args.repo_root)

        findings = hook_findings(full_path, root)
        if not findings:
            return 0

        lines = ["{file}:{line}: {kind} {rule} -- {detail}".format(**f) for f in findings]
        output = {
            "hookSpecificOutput": {
                "hookEventName": "PostToolUse",
                "additionalContext": "\n".join(lines),
            }
        }
        sys.stdout.write(json.dumps(output))
        return 0
    except BaseException:
        return 0


def main():
    parser = argparse.ArgumentParser(
        prog="comments",
        description=(
            "Comment linter and splice gate. 'check' reports MISSING sites that "
            "require a comment and INVALID comments that break a mechanical rule. "
            "'apply' splices a JSON array of proposals read from stdin."
        ),
    )
    parser.add_argument("--repo-root", default=os.getcwd())
    sub = parser.add_subparsers(dest="command", required=True)

    check = sub.add_parser("check", help="report MISSING sites and INVALID comments")
    check.add_argument("paths", nargs="*")
    check.add_argument("--base", default="HEAD", help="git ref to diff against (default HEAD)")
    check.add_argument("--all", action="store_true", help="scan whole files, not just changed lines")
    check.add_argument("--json", action="store_true")
    check.set_defaults(func=run_check)

    apply_cmd = sub.add_parser("apply", help="splice proposals read from stdin")
    apply_cmd.set_defaults(func=run_apply)

    hook_cmd = sub.add_parser(
        "hook",
        help="PostToolUse feedback hook -- reads a tool payload from stdin, reports findings for its file_path, always exits 0",
    )
    hook_cmd.set_defaults(func=run_hook)

    args = parser.parse_args()
    sys.exit(args.func(args))


if __name__ == "__main__":
    main()
