#!/usr/bin/env python3
from __future__ import annotations

import ast
import json
import os
import re
import subprocess
import sys

BUDGET = 2000
CAP = 5000
DMI_REGEX = r"^disable-model-invocation:\s*(true|yes|on|1)"
DMI = re.compile(DMI_REGEX, re.M | re.I)

HOOK_NAMES = ("comments", "git_guard", "verify_gate", "context_injector", "auto_format")

SKILL_KEYS = {
    "name", "description", "when_to_use", "model", "effort",
    "allowed-tools", "disallowed-tools", "argument-hint",
    "disable-model-invocation", "user-invocable", "paths",
    "context", "agent", "background", "hooks", "metadata", "shell",
    "license", "compatibility",
}
AGENT_KEYS = {
    "name", "description", "tools", "disallowedTools", "model",
    "permissionMode", "maxTurns", "skills", "mcpServers", "hooks",
    "memory", "background", "effort", "isolation", "color",
    "initialPrompt",
}

STALE = re.compile(r"\bTODO\.md\b")
INVOKE_VERB = re.compile(r"\b(?:invoke|invoking|invokes|dispatch|dispatches|dispatching)\b", re.I)
NAME = re.compile(r"[`/]([a-z][a-z0-9-]{2,})")
EXCLUDE_LINE = re.compile(r"\bexcludes?\b|on (?:their|its) own|the user invokes", re.I)

LANGUAGE_SKILLS = [
    "standards-go", "standards-typescript", "standards-python",
    "standards-c", "standards-vue", "standards-svelte", "standards-cdk",
    "standards-shell", "standards-dotnet", "standards-java", "standards-react",
]

ANCHORS = [
    "Comment discipline", "Toolchain", "Conventions", "Testing",
    "Quick-reference fields", "Repo layout", "Seed backlog",
    "Reference material",
]

HEADING = re.compile(r"^##\s+(.+?)\s*$", re.M)


def read_text(path: str, errors: str = "strict") -> str:
    with open(path, encoding="utf-8", errors=errors) as handle:
        return handle.read()


def listdir_sorted(path: str) -> list:
    if not os.path.isdir(path):
        return []
    return sorted(os.listdir(path))


def tokens(text: str) -> int:
    return max(1, len(text) // 4)


def check_links(source: str, target: str) -> int:
    problems = 0
    print("  links")
    for name in listdir_sorted(source):
        if name.startswith("."):
            continue
        entry = os.path.join(source, name)
        if not os.path.exists(entry):
            continue
        link = os.path.join(target, name)
        if os.path.islink(link) and os.path.exists(link):
            actual = os.readlink(link)
            if actual == entry:
                print("    ok        %s" % name)
            else:
                print("    stale     %s -> %s (expected %s)" % (name, actual, entry))
                problems += 1
        elif os.path.islink(link):
            print("    dangling  %s -> %s" % (name, os.readlink(link)))
            problems += 1
        elif os.path.exists(link):
            print("    not-link  %s (real file, run setup.sh)" % name)
            problems += 1
        else:
            print("    missing   %s (run setup.sh)" % name)
            problems += 1
    return problems


def check_hooks(source: str) -> int:
    problems = 0
    print("")
    print("  hooks")
    for hook in HOOK_NAMES:
        path = os.path.join(source, "hooks", hook + ".py")
        try:
            ast.parse(read_text(path))
        except (OSError, ValueError, SyntaxError):
            print("    BROKEN    %s.py does not parse" % hook)
            problems += 1
        else:
            print("    ok        %s.py" % hook)
    return problems


def frontmatter(path: str):
    body = read_text(path)
    if not body.startswith("---"):
        return None
    head = body.split("---", 2)
    if len(head) < 3:
        return None
    pairs = {}
    for line in head[1].splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if ":" not in line or line.startswith((" ", "\t", "-")):
            continue
        key, _, value = line.partition(":")
        pairs[key.strip()] = value.strip()
    return pairs


def check_frontmatter(source: str) -> int:
    print("")
    print("  frontmatter")
    failures = 0

    def one(path: str, label: str, allowed: set):
        count = 0
        fm = frontmatter(path)
        if fm is None:
            print("    BROKEN    %s has no YAML frontmatter" % label)
            return None, 1
        unknown = sorted(set(fm) - allowed)
        if unknown:
            print("    BROKEN    %s: unknown key(s) %s — the loader rejects the file" % (label, ", ".join(unknown)))
            count += 1
        for required in ("name", "description"):
            if not fm.get(required):
                print("    BROKEN    %s: missing %s" % (label, required))
                count += 1
        return fm, count

    skills_dir = os.path.join(source, "skills")
    for entry in listdir_sorted(skills_dir):
        path = os.path.join(skills_dir, entry, "SKILL.md")
        if not os.path.exists(path):
            continue
        fm, count = one(path, "skills/%s" % entry, SKILL_KEYS)
        failures += count
        if fm and fm.get("name") != entry:
            print("    BROKEN    skills/%s: name is '%s', must match the directory" % (entry, fm.get("name")))
            failures += 1

    agents_dir = os.path.join(source, "agents")
    for entry in listdir_sorted(agents_dir):
        if not entry.endswith(".md"):
            continue
        _, count = one(os.path.join(agents_dir, entry), "agents/%s" % entry, AGENT_KEYS)
        failures += count

    if failures == 0:
        print("    ok        every skill and agent parses against the loader schema")
    return 1 if failures else 0


def check_document_names(source: str, repo_root: str) -> int:
    print("")
    print("  document names")
    failures = 0
    for root in (source, os.path.join(repo_root, "templates")):
        for base, _, names in os.walk(root):
            for name in names:
                if not name.endswith((".md", ".tmpl", ".off")):
                    continue
                path = os.path.join(base, name)
                try:
                    body = read_text(path, errors="replace")
                except OSError:
                    continue
                if STALE.search(body):
                    label = os.path.relpath(path, repo_root)
                    print("    BROKEN    %s still names TODO.md — the file is BACKLOG.md" % label)
                    failures += 1
    if failures == 0:
        print("    ok        no skill or template names a renamed document")
    return 1 if failures else 0


def check_reachability(source: str) -> int:
    print("")
    print("  cross-skill reachability")
    skills_dir = os.path.join(source, "skills")
    unreachable = set()
    bodies = {}
    for entry in listdir_sorted(skills_dir):
        skill = os.path.join(skills_dir, entry, "SKILL.md")
        if not os.path.exists(skill):
            continue
        body = read_text(skill)
        bodies[entry] = body
        head = body.split("---")[1] if body.startswith("---") else ""
        if DMI.search(head):
            unreachable.add(entry)

    failures = 0
    for entry, body in bodies.items():
        for section in re.split(r"\n(?=#{1,6} )", body):
            if not INVOKE_VERB.search(section):
                continue
            for line in section.splitlines():
                if EXCLUDE_LINE.search(line):
                    continue
                for match in NAME.finditer(line):
                    target = match.group(1)
                    if target == entry or target not in unreachable:
                        continue
                    print("    BROKEN    %s invokes `%s`, which carries disable-model-invocation: true "
                          "and cannot be reached via the Skill tool" % (entry, target))
                    failures += 1

    if failures == 0:
        print("    ok        no skill body invokes a sibling it cannot reach")
    return 1 if failures else 0


def check_section_order(source: str) -> int:
    print("")
    print("  standards section order")
    failures = 0
    for name in LANGUAGE_SKILLS:
        path = os.path.join(source, "skills", name, "SKILL.md")
        if not os.path.exists(path):
            continue
        headings = HEADING.findall(read_text(path))
        indices = []
        for heading in headings:
            heading = heading.strip("`")
            for i, anchor in enumerate(ANCHORS):
                if heading == anchor or heading.startswith(anchor + " "):
                    indices.append((i, anchor))
                    break
        last_pre, in_layout, seen_reference, broken = -1, False, False, False
        for i, anchor in indices:
            if i <= 4:
                if in_layout or seen_reference or i < last_pre:
                    broken = True
                    break
                last_pre = i
            elif i in (5, 6):
                if seen_reference:
                    broken = True
                    break
                in_layout = True
            else:
                seen_reference = True
        if broken:
            print("    BROKEN    %s: section order violates the canonical shape" % name)
            failures += 1

    if failures == 0:
        print("    ok        every language/framework skill follows the canonical section order")
    return 1 if failures else 0


def check_hooks_path(repo_root: str) -> int:
    print("")
    print("  hooksPath")
    try:
        result = subprocess.run(
            ["git", "-C", repo_root, "config", "--get", "core.hooksPath"],
            capture_output=True,
        )
        hooks_path = result.stdout.decode("utf-8", "replace").strip()
    except (OSError, ValueError):
        hooks_path = ""

    if not hooks_path:
        print("    ok        core.hooksPath is unset")
        return 0
    if os.path.isdir(hooks_path) or os.path.isdir(os.path.join(repo_root, hooks_path)):
        print("    ok        core.hooksPath -> %s" % hooks_path)
        return 0
    print("    BROKEN    core.hooksPath -> %s does not resolve to a directory" % hooks_path)
    return 1


def check_budget(source: str, repo_root: str) -> int:
    print("")
    print("  base context budget")

    always_on = 0
    for name in ("AGENTS.md", "CLAUDE.md"):
        path = os.path.join(source, name)
        if os.path.exists(path):
            body = read_text(path)
            if body.strip() == "@AGENTS.md":
                continue
            count = tokens(body)
            always_on += count
            print("    %-24s %5d tokens" % (name, count))

    overrides = {}
    settings_path = os.path.join(source, "settings.json")
    if os.path.exists(settings_path):
        overrides = json.loads(read_text(settings_path)).get("skillOverrides", {})

    listing = 0
    skill_count = 0
    cap_failures = []
    paths_gated_count = 0
    paths_gated_tokens = 0
    skills_dir = os.path.join(source, "skills")
    for entry in listdir_sorted(skills_dir):
        skill = os.path.join(skills_dir, entry, "SKILL.md")
        if not os.path.exists(skill):
            continue
        body = read_text(skill)
        cap_count = tokens(body)
        if cap_count > CAP:
            cap_failures.append((entry, cap_count))
        head = body.split("---")[1] if body.startswith("---") else ""
        if DMI.search(head):
            continue
        state = overrides.get(entry, "on")
        if state in ("off", "user-invocable-only"):
            continue
        described = re.search(r"^description:\s*(.+)$", head, re.M)
        text = entry if state == "name-only" else entry + (described.group(1) if described else "")
        if re.search(r"^paths:", head, re.M):
            paths_gated_count += 1
            paths_gated_tokens += tokens(text)
            if state == "name-only":
                listing += tokens(entry)
                skill_count += 1
            continue
        listing += tokens(text)
        skill_count += 1

    print("    %-24s %5d tokens (%d listed to the model)" % ("skill listing", listing, skill_count))
    print("    %-24s %5d tokens (%d skills, gated by touched file, excluded from budget)" %
          ("paths:-gated skills", paths_gated_tokens, paths_gated_count))
    total = always_on + listing
    print("    %-24s %5d tokens" % ("TOTAL (always-on)", total))
    print()
    print("    NOTE: this total excludes bundled and plugin skills — skillOverrides is the only")
    print("    lever for bundled ones, /plugin for plugin ones; /context's Skills row is the real listing size.")
    over_budget = total > BUDGET
    if over_budget:
        print("    OVER BUDGET by %d tokens (limit %d)" % (total - BUDGET, BUDGET))
    else:
        print("    within budget (limit %d, %d free)" % (BUDGET, BUDGET - total))

    print()
    print("  skill compaction cap")
    for entry, cap_count in cap_failures:
        print("    BROKEN    skills/%s/SKILL.md: %d tokens, over the %d compaction re-attach cap" % (entry, cap_count, CAP))
    if not cap_failures:
        print("    ok        every SKILL.md is within the %d-token compaction re-attach cap" % CAP)

    if over_budget or cap_failures:
        return 1

    root_agents_path = os.path.join(repo_root, "AGENTS.md")
    if os.path.exists(root_agents_path):
        root_tokens = tokens(read_text(root_agents_path))
        print()
        print("    %-24s %5d tokens (this repo's project doc, loaded via CLAUDE.md's @AGENTS.md" % ("root AGENTS.md", root_tokens))
        print("                                    only while working in this repo — not part of the always-on budget above)")
        print("    %-24s %5d tokens" % ("TOTAL incl. project doc", total + root_tokens))
    return 0


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    source = os.path.join(repo_root, "claude-code")
    target = os.environ.get("AGENT_HOME") or os.path.join(os.path.expanduser("~"), ".claude")

    print()
    print("validate")
    print()

    problems = 0
    problems += check_links(source, target)
    problems += check_hooks(source)
    problems += check_frontmatter(source)
    problems += check_document_names(source, repo_root)
    problems += check_reachability(source)
    problems += check_section_order(source)
    problems += check_hooks_path(repo_root)
    problems += check_budget(source, repo_root)

    print()
    if problems == 0:
        print("  all checks passed")
        print()
        return 0
    print("  %d problem(s)" % problems)
    print()
    return 1


if __name__ == "__main__":
    sys.exit(main())
