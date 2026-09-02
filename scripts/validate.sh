#!/usr/bin/env bash
#
# validate.sh - health check for the installed config.
#
# Run:     bash scripts/validate.sh
# Exit 0:  everything healthy
# Exit 1:  one or more checks failed
#
# Checks, in order:
#   1. links        every claude-code/* is symlinked into ~/.claude
#   2. hooks        both guards parse as valid Python
#   3. frontmatter  every skill/agent uses only loader-known keys
#   4. hooksPath    core.hooksPath, if set, resolves to a real directory
#   5. budget       claude-code/AGENTS.md + skill listing under BUDGET tokens
#
# Check 3 is the one that matters most. The skill loader silently
# rejects any SKILL.md carrying a key it does not know, so a single
# stray key makes a skill vanish with no error anywhere. SKILL_KEYS
# in the frontmatter block below is the real allowlist.
#
# Tune: BUDGET is the always-on token ceiling, paid every turn of
# every session. Prefer moving content into a skill over raising it.
# The budget pass also reports repo-root AGENTS.md as a separate,
# ungated row — it costs tokens only while working in this repo (via
# CLAUDE.md's @AGENTS.md), not on every turn everywhere, so it is
# never folded into the BUDGET-gated total.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$REPO_ROOT/claude-code"
TARGET="${AGENT_HOME:-$HOME/.claude}"
BUDGET=2000

problems=0

printf '\nvalidate\n\n'
printf '  links\n'
for entry in "$SOURCE"/*; do
  [ -e "$entry" ] || continue
  name="$(basename "$entry")"
  link="$TARGET/$name"
  if [ -L "$link" ] && [ -e "$link" ]; then
    actual="$(cd "$(dirname "$link")" && readlink "$link")"
    if [ "$actual" = "$entry" ]; then
      printf '    ok        %s\n' "$name"
    else
      printf '    stale     %s -> %s (expected %s)\n' "$name" "$actual" "$entry"
      problems=$((problems + 1))
    fi
  elif [ -L "$link" ]; then
    printf '    dangling  %s -> %s\n' "$name" "$(readlink "$link")"
    problems=$((problems + 1))
  elif [ -e "$link" ]; then
    printf '    not-link  %s (real file, run setup.sh)\n' "$name"
    problems=$((problems + 1))
  else
    printf '    missing   %s (run setup.sh)\n' "$name"
    problems=$((problems + 1))
  fi
done

printf '\n  hooks\n'
for hook in comment_guard git_guard verify_gate context_injector auto_format; do
  path="$SOURCE/hooks/$hook.py"
  if python3 -c "import ast,sys; ast.parse(open(sys.argv[1]).read())" "$path" 2>/dev/null; then
    printf '    ok        %s.py\n' "$hook"
  else
    printf '    BROKEN    %s.py does not parse\n' "$hook"
    problems=$((problems + 1))
  fi
done

printf '\n  frontmatter\n'
python3 - "$SOURCE" <<'PY'
import os, sys

source = sys.argv[1]

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

failures = 0


def frontmatter(path):
    body = open(path, encoding="utf-8").read()
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


def check(path, label, allowed):
    global failures
    fm = frontmatter(path)
    if fm is None:
        print("    BROKEN    %s has no YAML frontmatter" % label)
        failures += 1
        return None
    unknown = sorted(set(fm) - allowed)
    if unknown:
        print("    BROKEN    %s: unknown key(s) %s — the loader rejects the file" % (label, ", ".join(unknown)))
        failures += 1
    for required in ("name", "description"):
        if not fm.get(required):
            print("    BROKEN    %s: missing %s" % (label, required))
            failures += 1
    return fm


skills_dir = os.path.join(source, "skills")
for entry in sorted(os.listdir(skills_dir)) if os.path.isdir(skills_dir) else []:
    path = os.path.join(skills_dir, entry, "SKILL.md")
    if not os.path.exists(path):
        continue
    fm = check(path, "skills/%s" % entry, SKILL_KEYS)
    if fm and fm.get("name") != entry:
        print("    BROKEN    skills/%s: name is '%s', must match the directory" % (entry, fm.get("name")))
        failures += 1

agents_dir = os.path.join(source, "agents")
for entry in sorted(os.listdir(agents_dir)) if os.path.isdir(agents_dir) else []:
    if not entry.endswith(".md"):
        continue
    check(os.path.join(agents_dir, entry), "agents/%s" % entry, AGENT_KEYS)

if failures == 0:
    print("    ok        every skill and agent parses against the loader schema")
sys.exit(1 if failures else 0)
PY
[ $? -eq 0 ] || problems=$((problems + 1))

printf '\n  document names\n'
python3 - "$SOURCE" "$REPO_ROOT" <<'PY'
import os, re, sys

source, repo_root = sys.argv[1], sys.argv[2]
STALE = re.compile(r"\bTODO\.md\b")
roots = [source, os.path.join(repo_root, "templates")]
failures = 0

for root in roots:
    for base, _, names in os.walk(root):
        for name in names:
            if not name.endswith((".md", ".tmpl", ".off")):
                continue
            path = os.path.join(base, name)
            try:
                with open(path, encoding="utf-8", errors="replace") as handle:
                    body = handle.read()
            except OSError:
                continue
            if STALE.search(body):
                label = os.path.relpath(path, repo_root)
                print("    BROKEN    %s still names TODO.md — the file is BACKLOG.md" % label)
                failures += 1

if failures == 0:
    print("    ok        no skill or template names a renamed document")
sys.exit(1 if failures else 0)
PY
[ $? -eq 0 ] || problems=$((problems + 1))

printf '\n  standards section order\n'
python3 - "$SOURCE" <<'PY'
import os, re, sys

source = sys.argv[1]

# Skills a .go/.py/.vue/etc glob loads. templates/skills/standards.md.tmpl
# names the canonical order this enforces; process/rule skills (cicd, sdlc,
# security, database, worklog, docs) are a different shape on purpose and
# are not in this list.
LANGUAGE_SKILLS = [
    "standards-go", "standards-typescript", "standards-python",
    "standards-c", "standards-vue", "standards-svelte", "standards-cdk",
    "standards-shell", "standards-dotnet", "standards-java", "standards-react",
]

# Anchors, in required relative order. A skill may omit any anchor (a
# scaffold-only skill has no Repo layout/Seed backlog; a base skill like
# standards-typescript has no Repo layout at all) — this checks that the
# anchors actually present never appear out of order, not that all exist.
ANCHORS = [
    "Comment discipline", "Toolchain", "Conventions", "Testing",
    "Quick-reference fields", "Repo layout", "Seed backlog",
    "Reference material",
]

HEADING = re.compile(r"^##\s+(.+?)\s*$", re.M)

failures = 0
for name in LANGUAGE_SKILLS:
    path = os.path.join(source, "skills", name, "SKILL.md")
    if not os.path.exists(path):
        continue
    headings = HEADING.findall(open(path, encoding="utf-8").read())
    indices = []
    for h in headings:
        h = h.strip("`")
        for i, anchor in enumerate(ANCHORS):
            if h == anchor or h.startswith(anchor + " "):
                indices.append((i, anchor))
                break
    # 0-4 (Comment discipline..Quick-reference fields) must be non-decreasing.
    # 5-6 (Repo layout, Seed backlog) may alternate any number of times —
    # one pair per repo type a skill supports (standards-go has go-api and
    # go-mcp). 7 (Reference material) must come after every such pair.
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
sys.exit(1 if failures else 0)
PY
[ $? -eq 0 ] || problems=$((problems + 1))

printf '\n  hooksPath\n'
hooks_path="$(git -C "$REPO_ROOT" config --get core.hooksPath 2>/dev/null || true)"
if [ -z "$hooks_path" ]; then
  printf '    ok        core.hooksPath is unset\n'
elif [ -d "$hooks_path" ] || [ -d "$REPO_ROOT/$hooks_path" ]; then
  printf '    ok        core.hooksPath -> %s\n' "$hooks_path"
else
  printf '    BROKEN    core.hooksPath -> %s does not resolve to a directory\n' "$hooks_path"
  problems=$((problems + 1))
fi

printf '\n  base context budget\n'
python3 - "$SOURCE" "$REPO_ROOT" "$BUDGET" <<'PY'
import os, re, sys

source, repo_root, budget = sys.argv[1], sys.argv[2], int(sys.argv[3])


def tokens(text):
    return max(1, len(text) // 4)


always_on = 0
for name in ("AGENTS.md", "CLAUDE.md"):
    path = os.path.join(source, name)
    if os.path.exists(path):
        body = open(path, encoding="utf-8").read()
        if body.strip() == "@AGENTS.md":
            continue
        count = tokens(body)
        always_on += count
        print("    %-24s %5d tokens" % (name, count))

overrides = {}
settings_path = os.path.join(source, "settings.json")
if os.path.exists(settings_path):
    import json
    overrides = json.load(open(settings_path, encoding="utf-8")).get("skillOverrides", {})

listing = 0
skills_dir = os.path.join(source, "skills")
skill_count = 0
if os.path.isdir(skills_dir):
    for entry in sorted(os.listdir(skills_dir)):
        skill = os.path.join(skills_dir, entry, "SKILL.md")
        if not os.path.exists(skill):
            continue
        body = open(skill, encoding="utf-8").read()
        head = body.split("---")[1] if body.startswith("---") else ""
        if re.search(r"^disable-model-invocation:\s*(true|yes|on|1)", head, re.M | re.I):
            continue
        state = overrides.get(entry, "on")
        if state in ("off", "user-invocable-only"):
            continue
        described = re.search(r"^description:\s*(.+)$", head, re.M)
        text = entry if state == "name-only" else entry + (described.group(1) if described else "")
        listing += tokens(text)
        skill_count += 1

print("    %-24s %5d tokens (%d listed to the model)" % ("skill listing", listing, skill_count))
total = always_on + listing
print("    %-24s %5d tokens" % ("TOTAL (always-on)", total))
print()
if total > budget:
    print("    OVER BUDGET by %d tokens (limit %d)" % (total - budget, budget))
    print("    NOTE: this total excludes bundled and plugin skills — skillOverrides is the only")
    print("    lever for bundled ones, /plugin for plugin ones; /context's Skills row is the real listing size.")
    sys.exit(1)
print("    within budget (limit %d, %d free)" % (budget, budget - total))
print("    NOTE: this total excludes bundled and plugin skills — skillOverrides is the only")
print("    lever for bundled ones, /plugin for plugin ones; /context's Skills row is the real listing size.")

root_agents_path = os.path.join(repo_root, "AGENTS.md")
if os.path.exists(root_agents_path):
    root_tokens = tokens(open(root_agents_path, encoding="utf-8").read())
    print()
    print("    %-24s %5d tokens (this repo's project doc, loaded via CLAUDE.md's @AGENTS.md" % ("root AGENTS.md", root_tokens))
    print("                                    only while working in this repo — not part of the always-on budget above)")
    print("    %-24s %5d tokens" % ("TOTAL incl. project doc", total + root_tokens))
PY
[ $? -eq 0 ] || problems=$((problems + 1))

printf '\n'
if [ "$problems" -eq 0 ]; then
  printf '  all checks passed\n\n'
  exit 0
fi
printf '  %d problem(s)\n\n' "$problems"
exit 1
