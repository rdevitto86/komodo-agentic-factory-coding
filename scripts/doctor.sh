#!/usr/bin/env bash
#
# doctor.sh - health check for the installed config.
#
# Run:     bash scripts/doctor.sh
# Exit 0:  everything healthy
# Exit 1:  one or more checks failed
#
# Checks, in order:
#   1. links        every home/* is symlinked into ~/.claude
#   2. hooks        all three guards parse as valid Python
#   3. frontmatter  every skill/agent uses only loader-known keys
#   4. budget       AGENTS.md + skill listing under BUDGET tokens
#
# Check 3 is the one that matters most. The skill loader silently
# rejects any SKILL.md carrying a key it does not know, so a single
# stray key makes a skill vanish with no error anywhere. SKILL_KEYS
# in the frontmatter block below is the real allowlist.
#
# Tune: BUDGET is the always-on token ceiling, paid every turn of
# every session. Prefer moving content into a skill over raising it.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$REPO_ROOT/home"
TARGET="$HOME/.claude"
BUDGET=2000

problems=0

printf '\ndoctor\n\n'
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
for hook in comment_guard scope_guard git_guard; do
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
    "name", "description", "model",
    "allowed-tools", "disallowed-tools", "argument-hint",
    "disable-model-invocation", "user-invocable",
}
AGENT_KEYS = {"name", "description", "tools", "model"}

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

printf '\n  base context budget\n'
python3 - "$SOURCE" "$BUDGET" <<'PY'
import os, re, sys

source, budget = sys.argv[1], int(sys.argv[2])


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
        described = re.search(r"^description:\s*(.+)$", head, re.M)
        listing += tokens(entry + (described.group(1) if described else ""))
        skill_count += 1

print("    %-24s %5d tokens (%d listed to the model)" % ("skill listing", listing, skill_count))
total = always_on + listing
print("    %-24s %5d tokens" % ("TOTAL", total))
print()
if total > budget:
    print("    OVER BUDGET by %d tokens (limit %d)" % (total - budget, budget))
    sys.exit(1)
print("    within budget (limit %d, %d free)" % (budget, budget - total))
PY
[ $? -eq 0 ] || problems=$((problems + 1))

printf '\n'
if [ "$problems" -eq 0 ]; then
  printf '  all checks passed\n\n'
  exit 0
fi
printf '  %d problem(s)\n\n' "$problems"
exit 1
