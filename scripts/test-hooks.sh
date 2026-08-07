#!/usr/bin/env bash
#
# test-hooks.sh - regression suite for the three guards.
#
# Run:     bash scripts/test-hooks.sh
# Exit 0:  every case passed
# Exit 1:  at least one failed, with the offending reason printed
#
# Each case feeds one hook a JSON payload on stdin and asserts the
# permissionDecision that comes back, optionally checking that the
# reason text does or does not contain a given string.
#
# Case IDs: C* comment guard, G* git guard, S* scope guard.
#
# Writing a case:
#   heredoc form  literal JSON, use \n for a newline inside a string
#   printf form   needed only when the payload must interpolate a
#                 shell variable; escape newlines as \\n there
#
# No hook may touch the real filesystem outside WORKDIR and the
# session state files the trap on line 8 removes.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOKS="$REPO_ROOT/home/hooks"
WORKDIR="$(mktemp -d)"
SESSION="hooktest$$"
trap 'rm -rf "$WORKDIR"; rm -f "${TMPDIR:-/tmp}/claude-scope-$SESSION.json" "${TMPDIR:-/tmp}/claude-comment-grant-$SESSION"' EXIT

RESULTS="$WORKDIR/results"
: > "$RESULTS"
HOOK=""

decision_of() {
  local out="$1"
  if [ -z "$out" ]; then
    printf 'allow'
    return
  fi
  printf '%s' "$out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["hookSpecificOutput"]["permissionDecision"])' 2>/dev/null \
    || printf 'malformed'
}

reason_of() {
  local out="$1"
  [ -z "$out" ] && return
  printf '%s' "$out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["hookSpecificOutput"]["permissionDecisionReason"])' 2>/dev/null
}

expect() {
  local label="$1" want="$2" must_contain="${3:-}" must_not_contain="${4:-}"
  local payload out got reason problem=""
  payload="$(cat)"
  out="$(printf '%s' "$payload" | python3 "$HOOK" 2>/dev/null)"
  got="$(decision_of "$out")"
  reason="$(reason_of "$out")"

  [ "$got" != "$want" ] && problem="decision=$got want=$want"
  if [ -z "$problem" ] && [ -n "$must_contain" ] && [[ "$reason" != *"$must_contain"* ]]; then
    problem="reason missing: $must_contain"
  fi
  if [ -z "$problem" ] && [ -n "$must_not_contain" ] && [[ "$reason" == *"$must_not_contain"* ]]; then
    problem="reason leaked: $must_not_contain"
  fi

  if [ -z "$problem" ]; then
    printf '  PASS  %s\n' "$label"
    printf 'PASS\n' >> "$RESULTS"
  else
    printf '  FAIL  %s\n        %s\n' "$label" "$problem"
    [ -n "$reason" ] && printf '%s\n' "$reason" | sed 's/^/        | /'
    printf 'FAIL\n' >> "$RESULTS"
  fi
}

bash_case() {
  local label="$1" want="$2" command="$3" must_contain="${4:-}"
  printf '%s' "$command" \
    | python3 -c 'import json,sys; print(json.dumps({"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":sys.stdin.read()}}))' \
    | expect "$label" "$want" "$must_contain"
}

# ─────────────────────────────  comment guard  ─────────────────────────────
HOOK="$HOOKS/comment_guard.py"
printf '\ncomment guard\n\n'

expect "C1  new comment beside user block flags only the new line" deny \
  "agent added this" "user note one" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"// user note one\n// user note two\nfunc Foo() {","new_string":"// user note one\n// user note two\n// agent added this\nfunc Foo() {"}}
JSON

expect "C2  trailing comment survives an edit to its own line" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"x := 1 << 20 // 1MB","new_string":"x := 1 << 21 // 1MB"}}
JSON

expect "C3  deleting a user comment asks instead of proceeding" ask \
  "user note" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"// user note\nfunc Foo() {}","new_string":"func Foo() {}"}}
JSON

expect "C4  MultiEdit cannot smuggle a comment through" deny \
  "smuggled" <<'JSON'
{"tool_name":"MultiEdit","tool_input":{"file_path":"/x/svc.go","edits":[{"old_string":"a := 1","new_string":"a := 2"},{"old_string":"b := 1","new_string":"// smuggled\nb := 2"}]}}
JSON

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/fresh.go","content":"// package header\\npackage main\\n"}}' "$WORKDIR" \
  | expect "C5  Write to a brand-new file denies its comments" deny "package header"

expect "C6a machine directives are exempt (go)" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"package main","new_string":"//go:build linux\n//nolint:gocyclo\npackage main"}}
JSON

expect "C6b machine directives are exempt (shell)" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/run.sh","old_string":"echo hi","new_string":"#!/usr/bin/env bash\n# shellcheck disable=SC2086\necho hi"}}
JSON

expect "C7  python docstrings are comments and are denied" deny \
  "Return one" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/mod.py","old_string":"def f():\n    return 1","new_string":"def f():\n    \"\"\"Return one.\"\"\"\n    return 1"}}
JSON

expect "C8  malformed payload fails closed" deny \
  "Failing closed" <<'JSON'
{"tool_name":"Edit","tool_input":
JSON

expect "C9  a // inside a string literal is not a comment" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"host := \"\"","new_string":"host := \"https://api.example.com\""}}
JSON

expect "C10 reindenting an existing comment is not a change" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"// kept verbatim\nif x {","new_string":"\tif x {\n\t\t// kept verbatim"}}
JSON

expect "C11 unsupported file types are ignored" allow <<'JSON'
{"tool_name":"Write","tool_input":{"file_path":"/x/notes.md","content":"<!-- a markdown note -->\n# Title\n"}}
JSON

expect "C12 docstring survives a body-only edit" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/mod.py","old_string":"def f():\n    \"\"\"Return one.\"\"\"\n    return 1","new_string":"def f():\n    \"\"\"Return one.\"\"\"\n    return 2"}}
JSON

expect "C13 sql line comments are caught" deny "backfill" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/001_init.sql","old_string":"ALTER TABLE orders ADD COLUMN tenant_id uuid;","new_string":"-- backfill later\nALTER TABLE orders ADD COLUMN tenant_id uuid;"}}
JSON

expect "C14 shebang header block is exempt" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/doctor.sh","old_string":"#!/usr/bin/env bash\nset -e","new_string":"#!/usr/bin/env bash\n#\n# doctor.sh - health check.\n#\n# Run: bash doctor.sh\n\nset -e"}}
JSON

expect "C15 a shebang does not exempt the rest of the file" deny \
  "retry twice" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/doctor.sh","old_string":"#!/usr/bin/env bash\n# manual line\n\nrun_checks","new_string":"#!/usr/bin/env bash\n# manual line\n\n# retry twice\nrun_checks"}}
JSON

expect "C16 a header block without a shebang is still denied" deny \
  "orders service" <<'JSON'
{"tool_name":"Write","tool_input":{"file_path":"/x/svc.go","content":"// orders service\npackage main\n"}}
JSON

grant_prompt() {
  printf '{"hook_event_name":"UserPromptSubmit","session_id":"%s","prompt":%s}' "$SESSION" "$1"
}

grant_prompt '"tidy this up please"' | python3 "$HOOK" >/dev/null 2>&1
printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"// explain Foo\\nfunc Foo() {}"}}' "$SESSION" \
  | expect "C17 without the sigil a comment is denied" deny "explain Foo"

grant_prompt '"add a header here +comments"' | python3 "$HOOK" >/dev/null 2>&1
printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"// explain Foo\\nfunc Foo() {}"}}' "$SESSION" \
  | expect "C18 +comments in the user prompt grants the turn" allow

grant_prompt '"now carry on"' | python3 "$HOOK" >/dev/null 2>&1
printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Bar() {}","new_string":"// explain Bar\\nfunc Bar() {}"}}' "$SESSION" \
  | expect "C19 the grant expires on the next prompt" deny "explain Bar"

grant_prompt '"add a header here +comments"' | python3 "$HOOK" >/dev/null 2>&1
printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"// user note\\nfunc Foo() {}","new_string":"func Foo() {}"}}' "$SESSION" \
  | expect "C20 a grant never licenses deleting a comment" ask "user note"

printf '{"hook_event_name":"PreToolUse","session_id":"%s-other","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"// +comments\\n// explain Foo\\nfunc Foo() {}"}}' "$SESSION" \
  | expect "C21 the agent cannot grant itself by writing the sigil" deny "explain Foo"

grant_prompt '"reset"' | python3 "$HOOK" >/dev/null 2>&1

# ─────────────────────────────────  git guard  ─────────────────────────────
HOOK="$HOOKS/git_guard.py"
printf '\ngit guard\n\n'

bash_case "G1  git commit is blocked"                deny  'git commit -m "wip"'          "changes repository state"
bash_case "G2  git log is allowed"                   allow 'git log --oneline -20'
bash_case "G3  git restore is blocked"               deny  'git restore src/main.go'      "git restore"
bash_case "G4  git checkout -- is blocked"           deny  'git checkout -- src/main.go'  "git checkout"
bash_case "G5  echo of a git string is not a match"  allow 'echo "git commit -m x"'
bash_case "G6  sh -c wrapper is unwrapped"           deny  'sh -c "git push origin main"' "git push"
bash_case "G7  sed -i is blocked"                    deny  "sed -i '' 's/a/b/' main.go"   "bypassing the comment guard"
bash_case "G8  redirect into a code file is blocked" deny  'cat > handler.go'             "bypasses the comment guard"
bash_case "G9  chained read-only git is allowed"     allow 'git status && git diff --stat'
bash_case "G10 chained commit is caught"             deny  'git diff && git commit -m x'  "git commit"
bash_case "G11 bare git branch lists and is allowed" allow 'git branch -a'
bash_case "G12 creating a branch is blocked"         deny  'git branch feature/x'         "creates a branch"
bash_case "G13 git stash is blocked"                 deny  'git stash'                    "changes repository state"
bash_case "G14 git fetch is allowed"                 allow 'git fetch origin'
bash_case "G15 git reset --hard is blocked"          deny  'git reset --hard HEAD~1'      "git reset"
bash_case "G16 unrelated commands pass"              allow 'go test ./... && npm run build'
bash_case "G17 redirect to a non-code file passes"   allow 'go test ./... > /tmp/out.txt'
bash_case "G19 git stash list is allowed"            allow 'git stash list'
bash_case "G20 git config set is blocked"            deny  'git config user.email a@b.c' "sets a value"
bash_case "G21 git config get is allowed"            allow 'git config --get user.email'
bash_case "G22 perl -i is blocked"                   deny  "perl -pi -e 's/a/b/' main.go" "bypassing the comment guard"
bash_case "G23 append redirect to code is blocked"   deny  'echo x >> util.ts'           "bypasses the comment guard"
bash_case "G18 git clean is blocked"                 deny  'git clean -fdx'               "git clean"

# ────────────────────────────────  scope guard  ────────────────────────────
HOOK="$HOOKS/scope_guard.py"
printf '\nscope guard\n\n'

scope_payload() {
  python3 -c "import json,sys; print(json.dumps({'hook_event_name':sys.argv[1],'session_id':sys.argv[2],'tool_name':'Edit','tool_input':{'file_path':sys.argv[3]}}))" "$1" "$SESSION" "${2:-/x/a.go}"
}

scope_payload UserPromptSubmit | python3 "$HOOK" >/dev/null 2>&1

scope_payload PreToolUse /x/a.go | expect "S1  first file of the turn is allowed" allow
scope_payload PostToolUse /x/a.go | python3 "$HOOK" >/dev/null 2>&1
scope_payload PreToolUse /x/a.go | expect "S2  same file again is allowed" allow
scope_payload PreToolUse /x/b.go | expect "S3  second distinct file asks" ask "a.go"
scope_payload UserPromptSubmit | python3 "$HOOK" >/dev/null 2>&1
scope_payload PreToolUse /x/b.go | expect "S4  new turn resets the budget" allow

PASS="$(grep -c '^PASS$' "$RESULTS" || true)"
FAIL="$(grep -c '^FAIL$' "$RESULTS" || true)"
printf '\n  %d passed, %d failed\n\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
