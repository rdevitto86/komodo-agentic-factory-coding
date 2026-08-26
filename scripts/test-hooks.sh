#!/usr/bin/env bash
#
# test-hooks.sh - regression suite for the two guards.
#
# Run:     bash scripts/test-hooks.sh
# Exit 0:  every case passed
# Exit 1:  at least one failed, with the offending reason printed
#
# Each case feeds one hook a JSON payload on stdin and asserts the
# permissionDecision that comes back, optionally checking that the
# reason text does or does not contain a given string.
#
# Case IDs: C* comment guard, G* git guard.
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
HOOKS="$REPO_ROOT/claude-code/hooks"
WORKDIR="$(mktemp -d)"
SESSION="hooktest$$"
trap 'rm -rf "$WORKDIR"; rm -f "${TMPDIR:-/tmp}/claude-comment-grant-$SESSION" "${TMPDIR:-/tmp}/claude-comment-ledger-$SESSION" "${TMPDIR:-/tmp}/claude-comment-ledger-$SESSION-move"' EXIT

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
  payload="$(printf '%s' "$payload" | python3 -c '
import json, sys
raw = sys.stdin.read()
try:
    p = json.loads(raw)
except ValueError:
    sys.stdout.write(raw); raise SystemExit(0)
p.setdefault("hook_event_name", "PreToolUse")
sys.stdout.write(json.dumps(p))
')"
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

# git_guard.py's current_branch() shells out to `git rev-parse` in the
# hook's own cwd, so any case whose outcome depends on the branch needs
# a real fixture repo on a known branch, not this checkout's real one.
FIXTURE_MAIN="$WORKDIR/fixture-main"
FIXTURE_FEAT="$WORKDIR/fixture-feat"
git init -q -b main "$FIXTURE_MAIN"
(cd "$FIXTURE_MAIN" && git config user.email t@t.com && git config user.name t && git commit -q --allow-empty -m init)
git init -q -b main "$FIXTURE_FEAT"
(cd "$FIXTURE_FEAT" && git config user.email t@t.com && git config user.name t && git commit -q --allow-empty -m init && git checkout -q -b feat/test-branch)

bash_case_at() {
  local dir="$1" label="$2" want="$3" command="$4" must_contain="${5:-}"
  local payload out got reason problem=""
  payload="$(printf '%s' "$command" | python3 -c 'import json,sys; print(json.dumps({"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":sys.stdin.read()}}))')"
  out="$(cd "$dir" && printf '%s' "$payload" | python3 "$HOOK" 2>/dev/null)"
  got="$(decision_of "$out")"
  reason="$(reason_of "$out")"
  [ "$got" != "$want" ] && problem="decision=$got want=$want"
  if [ -z "$problem" ] && [ -n "$must_contain" ] && [[ "$reason" != *"$must_contain"* ]]; then
    problem="reason missing: $must_contain"
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

# ─────────────────────────────  comment guard  ─────────────────────────────
HOOK="$HOOKS/comment_guard.py"
printf '\ncomment guard\n\n'

expect "C1  new comment beside user block flags only the new line" ask \
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

expect "C4  MultiEdit cannot smuggle a comment through" ask \
  "smuggled" <<'JSON'
{"tool_name":"MultiEdit","tool_input":{"file_path":"/x/svc.go","edits":[{"old_string":"a := 1","new_string":"a := 2"},{"old_string":"b := 1","new_string":"// smuggled\nb := 2"}]}}
JSON

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/fresh.go","content":"// package header\\npackage main\\n"}}' "$WORKDIR" \
  | expect "C5  Write to a brand-new file asks about its comments" ask "package header"

expect "C6a machine directives are exempt (go)" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"package main","new_string":"//go:build linux\n//nolint:gocyclo\npackage main"}}
JSON

expect "C6b machine directives are exempt (shell)" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/run.sh","old_string":"echo hi","new_string":"#!/usr/bin/env bash\n# shellcheck disable=SC2086\necho hi"}}
JSON

expect "C7  python docstrings are comments and prompt for approval" ask \
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

expect "C13 sql line comments are caught" ask "backfill" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/001_init.sql","old_string":"ALTER TABLE orders ADD COLUMN tenant_id uuid;","new_string":"-- backfill later\nALTER TABLE orders ADD COLUMN tenant_id uuid;"}}
JSON

expect "C14 shebang header block is exempt" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/doctor.sh","old_string":"#!/usr/bin/env bash\nset -e","new_string":"#!/usr/bin/env bash\n#\n# doctor.sh - health check.\n#\n# Run: bash doctor.sh\n\nset -e"}}
JSON

expect "C15 a shebang does not exempt the rest of the file" ask \
  "retry twice" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/doctor.sh","old_string":"#!/usr/bin/env bash\n# manual line\n\nrun_checks","new_string":"#!/usr/bin/env bash\n# manual line\n\n# retry twice\nrun_checks"}}
JSON

expect "C16 a header block without a shebang still prompts" ask \
  "orders service" <<'JSON'
{"tool_name":"Write","tool_input":{"file_path":"/x/svc.go","content":"// orders service\npackage main\n"}}
JSON

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"// explain Foo\\nfunc Foo() {}"}}' "$SESSION" \
  | expect "C17 a doc comment on a func prompts for approval" ask "explain Foo"

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Bar() {}","new_string":"// explain Bar\\nfunc Bar() {}"}}' "$SESSION" \
  | expect "C19 an ordinary doc comment prompts for approval" ask "explain Bar"

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"// user note\\nfunc Foo() {}","new_string":"func Foo() {}"}}' "$SESSION" \
  | expect "C20 deleting a comment asks, never proceeds" ask "user note"

printf '{"hook_event_name":"PreToolUse","session_id":"%s-other","tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"// +comments\\n// explain Foo\\nfunc Foo() {}"}}' "$SESSION" \
  | expect "C21 the agent cannot grant itself by writing the sigil" ask "explain Foo"

expect "C22 the Helpers banner is exempt in a test file" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc_test.go","old_string":"func TestFoo(t *testing.T) {}","new_string":"func TestFoo(t *testing.T) {}\n\n// --- Helpers ----------------------------------------------------\n\nfunc newFixture(t *testing.T) {}"}}
JSON

expect "C23 any --- Label --- banner is a section break" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc_test.go","old_string":"func TestFoo(t *testing.T) {}","new_string":"func TestFoo(t *testing.T) {}\n\n// --- Setup ---\n\nfunc newFixture(t *testing.T) {}"}}
JSON

expect "C24 a banner is allowed outside a test path too" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"func Foo() {}\n\n// --- Helpers ---\n\nfunc bar() {}"}}
JSON

expect "C25 a description above a Go test has no slot" ask \
  "shipped branch" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestCancel(t *testing.T) {}","new_string":"// the shipped branch is unreachable through the public API\nfunc TestCancel(t *testing.T) {}"}}
JSON

expect "C26 the same text above a non-test func prompts" ask \
  "unreachable" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func newOrder() {}","new_string":"// the shipped branch is unreachable through the public API\nfunc newOrder() {}"}}
JSON

expect "C27 a three-line description exceeds the cap" ask \
  "third sentence" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestCancel(t *testing.T) {}","new_string":"// first sentence on the boundary\n// second sentence on the boundary\n// third sentence on the boundary\nfunc TestCancel(t *testing.T) {}"}}
JSON

expect "C28 a description over 200 characters prompts" ask \
  "config default" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestCancel(t *testing.T) {}","new_string":"// the retry ceiling interacts with the jitter window in a way the public API cannot express, so this case pins the exact boundary that a reader would otherwise have to derive from three separate files and a config default\nfunc TestCancel(t *testing.T) {}"}}
JSON

expect "C29 a blank line breaks the description slot" ask \
  "degraded default" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestZero(t *testing.T) {}","new_string":"// checks the degraded default\n\nfunc TestZero(t *testing.T) {}"}}
JSON

expect "C30 a block comment above a test prompts" ask \
  "degraded default" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestZero(t *testing.T) {}","new_string":"/* checks the degraded default */\nfunc TestZero(t *testing.T) {}"}}
JSON

expect "C31 a plain helper file under test/ gets the banner" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/test/integration/helpers.go","old_string":"func Seed() {}","new_string":"func Seed() {}\n\n// --- Helpers ---\n\nfunc reset() {}"}}
JSON

expect "C32 an arbitrary comment in a test file still prompts" ask \
  "bump the counter" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"count++","new_string":"// bump the counter\ncount++"}}
JSON

expect "C32b a description above a truncated signature prompts" ask \
  "cookie is only checked" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order_test.go","old_string":"func TestNew","new_string":"// the cookie is only checked on unsafe methods\nfunc TestNew"}}
JSON

expect "C33 a description above a pytest test prompts" ask \
  "retry ceiling" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/test_order.py","old_string":"def test_retry(n):\n    pass","new_string":"# pins the retry ceiling at the documented boundary\n@pytest.mark.parametrize(\"n\", [1, 2])\ndef test_retry(n):\n    pass"}}
JSON

expect "C34 a description above a Rust #[test] prompts" ask \
  "saturating add" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/tests/limits.rs","old_string":"fn saturates_at_cap() {}","new_string":"// the saturating add is only reachable at usize::MAX\n#[test]\nfn saturates_at_cap() {}"}}
JSON

expect "C35 a description above a Vitest case prompts" ask \
  "402 branch" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/order.x.test.ts","old_string":"it(\"rejects an expired card\", () => {","new_string":"// the 402 branch cannot be reached through the public client\nit(\"rejects an expired card\", () => {"}}
JSON

expect "C36 a comment restating the func name prompts as an echo" ask \
  "restates the name" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/store.go","old_string":"func InitStore() {}","new_string":"// InitStore inits a store\nfunc InitStore() {}"}}
JSON

expect "C36b the same rule catches a Go method receiver" ask \
  "restates the name" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/sink.go","old_string":"func (s *Sink) Send(e Event) error {","new_string":"// Send sends the event to the HTTP sink.\nfunc (s *Sink) Send(e Event) error {"}}
JSON

expect "C36c the same rule catches a TypeScript class" ask \
  "restates the name" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/cart.ts","old_string":"export class CartService {","new_string":"// CartService is a service for carts\nexport class CartService {"}}
JSON

expect "C37 a step marker inside a body is allowed" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {\n\tboot()","new_string":"func Foo() {\n\t// init runtime logger\n\tboot()"}}
JSON

expect "C38 a step marker over 80 chars prompts" ask \
  "self-documenting" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {\n\tboot()","new_string":"func Foo() {\n\t// initialise the runtime logger so that downstream calls can resolve their credentials from AWS\n\tboot()"}}
JSON

expect "C39 a step marker at column zero prompts" ask \
  "init runtime logger" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"var boot = 1","new_string":"// init runtime logger\nvar boot = 1"}}
JSON

expect "C40 a multi-line run inside a body prompts" ask \
  "self-documenting" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {\n\tboot()","new_string":"func Foo() {\n\t// init the logger\n\t// then boot\n\tboot()"}}
JSON

expect "C41 a box-glyph banner is not a banner" ask \
  "Helpers" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc_test.go","old_string":"func TestFoo(t *testing.T) {}","new_string":"func TestFoo(t *testing.T) {}\n\n// ── Helpers ──────────────\n\nfunc newFixture(t *testing.T) {}"}}
JSON

expect "C42 a banner label over 40 chars prompts" ask \
  "Helpers" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc_test.go","old_string":"func TestFoo(t *testing.T) {}","new_string":"func TestFoo(t *testing.T) {}\n\n// --- Helpers: shared fixtures, builders and assertion utilities ---\n\nfunc newFixture(t *testing.T) {}"}}
JSON

expect "C43 a bare rule with no label is not a banner" ask \
  "self-documenting" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"func Foo() {}\n\n// ------------------------------\n\nfunc bar() {}"}}
JSON

expect "C44 a banner cannot smuggle prose past the hyphens" ask \
  "self-documenting" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/svc.go","old_string":"func Foo() {}","new_string":"func Foo() {}\n\n// --- this helper exists because the upstream client retries twice ---\n\nfunc bar() {}"}}
JSON

MOVE="$SESSION-move"
printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/from.go","old_string":"// Preloads all the required dependencies.\\nfunc Boot() {}","new_string":""}}' "$MOVE" \
  | expect "C45 removing a comment asks and records it" ask "Preloads all the required"

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/to.go","old_string":"package x","new_string":"package x\\n\\n// Preloads all the required dependencies.\\nfunc Boot() {}"}}' "$MOVE" \
  | expect "C46 re-adding it at the destination still prompts" ask "Preloads all the required"

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/store.go","old_string":"func InitStore() {}","new_string":"// InitStore inits a store\\nfunc InitStore() {}"}}' "$SESSION" \
  | expect "C49 a name-echo prompts as an echo" ask "restates the name"

printf '{"hook_event_name":"PreToolUse","session_id":"%s","tool_name":"Edit","tool_input":{"file_path":"/x/store.go","old_string":"func InitStore() {}","new_string":"// Creates the store and seeds it from disk.\\nfunc InitStore() {}"}}' "$SESSION" \
  | expect "C50 an ordinary doc comment prompts for approval" ask "self-documenting"

expect "C51 a script manual under a shebang is still exempt" allow <<'JSON'
{"tool_name":"Write","tool_input":{"file_path":"/x/deploy.sh","content":"#!/usr/bin/env bash\n#\n# deploy.sh - ships the current build to staging.\n#\n# Usage:  bash deploy.sh [--dry-run]\n#\n# Exit 0: shipped\n# Exit 1: refused\nset -euo pipefail\n"}}
JSON

expect "C53 an HTML narrative comment in a Vue template prompts" ask <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/Widget.vue","old_string":"<div>hi</div>","new_string":"<!-- explains the widget --> \n<div>hi</div>"}}
JSON

expect "C54 a WHY note in a Svelte template's HTML comment is allowed" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/Widget.svelte","old_string":"<div>hi</div>","new_string":"<!-- WHY: Safari needs this wrapper -->\n<div>hi</div>"}}
JSON

expect "C55 a banner in an HTML comment is allowed" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/Widget.vue","old_string":"<div>hi</div>","new_string":"<!-- --- Header --- -->\n<div>hi</div>"}}
JSON

expect "C52 a manual not under a shebang prompts" ask \
  "ships the current build" <<'JSON'
{"tool_name":"Write","tool_input":{"file_path":"/x/deploy.sh","content":"set -euo pipefail\n#\n# deploy.sh - ships the current build to staging.\n#\necho hi\n"}}
JSON

expect "C56 a pre-existing echo comment already in old_string doesn't re-prompt" allow <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/store.go","old_string":"// InitStore inits a store\nfunc InitStore() {}\nfoo()","new_string":"// InitStore inits a store\nfunc InitStore() {}\nbar()"}}
JSON

expect "C56b a new echo comment on an untouched neighbor line still prompts" ask \
  "restates the name" <<'JSON'
{"tool_name":"Edit","tool_input":{"file_path":"/x/store.go","old_string":"foo()","new_string":"// InitStore inits a store\nfunc InitStore() {}\nfoo()"}}
JSON

printf '// Helper does the work\nfunc Other() {}\nfoo()\n' > "$WORKDIR/helper.go"
printf '{"tool_name":"Edit","tool_input":{"file_path":"%s/helper.go","old_string":"foo()","new_string":"// Helper does the work\\nfunc Helper() {}\\nfoo()"}}' "$WORKDIR" \
  | expect "C56c an echo comment whose exact text already exists elsewhere in the file still prompts" ask "restates the name"

# ─────────────────────────────────  git guard  ─────────────────────────────
HOOK="$HOOKS/git_guard.py"
printf '\ngit guard\n\n'

bash_case_at "$FIXTURE_MAIN" "G1a committing directly on a protected branch is blocked" deny 'git commit -m "wip"' "create a branch first"
bash_case_at "$FIXTURE_FEAT" "G1b committing on a feature branch is allowed"            allow 'git commit -m "wip"'
bash_case "G2  git log is allowed"                   allow 'git log --oneline -20'
bash_case "G3  git restore is blocked"               deny  'git restore src/main.go'      "git restore"
bash_case "G4  git checkout -- is blocked"           deny  'git checkout -- src/main.go'  "git checkout"
bash_case "G5  echo of a git string is not a match"  allow 'echo "git commit -m x"'
bash_case "G6  sh -c wrapper is unwrapped"           deny  'sh -c "git push origin main"' "open a pull request instead"
bash_case "G7  sed -i is blocked"                    deny  "sed -i '' 's/a/b/' main.go"   "bypassing the comment guard"
bash_case "G8  redirect into a code file is blocked" deny  'cat > handler.go'             "bypasses the comment guard"
bash_case "G9  chained read-only git is allowed"     allow 'git status && git diff --stat'
bash_case_at "$FIXTURE_MAIN" "G10 chained commit on a protected branch is caught" deny 'git diff && git commit -m x' "create a branch first"
bash_case "G11 bare git branch lists and is allowed" allow 'git branch -a'
bash_case "G12a creating a conventionally-named branch is allowed" allow 'git branch feat/x'
bash_case "G12b creating a badly-named branch is blocked"          deny  'git branch feature/x' "kebab-case"
bash_case "G12c naming a protected branch is blocked"              deny  'git branch main'       "kebab-case"
bash_case "G13a bare git stash (push-equivalent) is allowed" allow 'git stash'
bash_case "G13b git stash drop is blocked"                   deny  'git stash drop' "discards saved state"
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
bash_case "G24 an ASCII arrow is not a redirect"     allow 'echo "step 1 -> main.go done"'
bash_case "G25 the word tee in a string is not tee"  allow 'echo "redirect/tee cases" && ls scripts/test-hooks.sh'
bash_case "G26 real tee into a code file is blocked" deny  'go test ./... | tee results.go' "tee writing to"
bash_case "G27 a quoted redirect is data"            allow 'echo "write it with > handler.go"'
bash_case "G28 a heredoc body is data"               allow 'python3 - <<'"'"'PY'"'"'
print("emit > handler.go here")
PY'
bash_case "G29 a redirect on the heredoc opener still blocks" deny 'cat <<'"'"'EOF'"'"' > handler.go
package main
EOF' "bypasses the comment guard"
bash_case "G30 numeric fd redirect is not a code path" allow 'go build ./... 2>&1'
bash_case "G31 cp over a code path is blocked"       deny  'cp /tmp/staged.go handler.go'  "bypasses the comment guard"
bash_case "G32 mv over a code path is blocked"       deny  'mv /tmp/staged.go handler.go'  "bypasses the comment guard"
bash_case "G33 cp between non-code paths is allowed" allow 'cp /tmp/a.txt /tmp/b.txt'
bash_case "G34 mv of a directory listing is allowed" allow 'mv build/ dist/'
bash_case "G35 time git rebase is caught"            deny  'time git rebase main'          "git rebase"
bash_case "G36 command git rebase is caught"         deny  'command git rebase main'        "git rebase"
bash_case "G37 xargs git rebase is caught"           deny  'xargs -I{} git rebase main'     "git rebase"
bash_case "G38 nohup git push is caught"             deny  'nohup git push origin main &'   "git push"
bash_case "G39 eval of a git string is caught"       deny  'eval "git rebase main"'         "git rebase"
bash_case "G40 time go test is allowed"              allow 'time go test ./...'
mkdir -p "$WORKDIR/somedir"
bash_case "G41 an unlisted long value-flag on a passthrough wrapper doesn't hide the wrapped command" \
  deny  'time --output logfile.py git rebase main' "git rebase"
bash_case "G42 mv into a trailing-slash directory destination is blocked" \
  deny  'mv payload.py somedir/' "bypasses the comment guard"
bash_case "G43 cp into an existing bare-name directory destination is blocked" \
  deny  "cp payload.py $WORKDIR/somedir" "bypasses the comment guard"
bash_case "G44 cp -t names the real target out of position" \
  deny  'cp -t module.py staged.txt' "bypasses the comment guard"
bash_case "G45 cp -t DIR still checks the sources being placed" \
  deny  'cp -t somedir payload.go' "bypasses the comment guard"
bash_case "G46 mv -t DIR/ still checks the sources being placed" \
  deny  'mv -t assets/ handler.go' "bypasses the comment guard"
bash_case "G47 mv into a directory created earlier in the same command" \
  deny  'mkdir -p brandnewdir && mv payload.go brandnewdir' "bypasses the comment guard"
bash_case "G48 git tag creating an annotated tag is allowed" \
  allow 'git tag -a v1.2.3 abc123 -m "release"'
bash_case "G49 git tag creating a lightweight tag is allowed" \
  allow 'git tag v1.2.3 abc123'
bash_case "G50 git tag -d is blocked"                deny  'git tag -d v1.2.3'            "changes repository state"
bash_case "G51 git tag -f is blocked"                deny  'git tag -f v1.2.3 abc123'     "changes repository state"
bash_case "G52 git switch -c with a conventional name is allowed" allow 'git switch -c feat/redesign'
bash_case "G53 git switch -c with a bad name is blocked"          deny  'git switch -c badname' "kebab-case"
bash_case "G54 git switch -c naming a protected branch is blocked" deny 'git switch -c main'    "kebab-case"
bash_case "G55 git switch to an existing branch is allowed"      allow 'git switch main'
bash_case "G56 git switch --detach is blocked"                    deny  'git switch --detach HEAD' "is denied"
bash_case_at "$FIXTURE_FEAT" "G57 git push on a feature branch is allowed" allow 'git push -u origin feat/test-branch'
bash_case "G58 git push to main is blocked"                      deny  'git push origin main' "open a pull request instead"
bash_case "G59 git push --force is blocked"                      deny  'git push --force origin feat/x' "rewrites published history"
bash_case "G60 git add -A is allowed"                            allow 'git add -A'
bash_case "G61 git add -p is blocked"                            deny  'git add -p' "interactive"
bash_case "G62 a commit with a co-author trailer is blocked" deny 'git commit -m "fix: x

Co-Authored-By: bot <b@b.com>"' "trailer"
bash_case "G63 git commit --amend is blocked"                    deny  'git commit --amend -m x' "rewrites a commit"
bash_case "G64 gh pr create is allowed"                          allow 'gh pr create --title x --body y'
bash_case "G65 gh pr merge is blocked"                           deny  'gh pr merge 5' "is denied"
bash_case "G66 gh label create is blocked"                       deny  'gh label create foo' "is denied"
bash_case "G67 gh label list is allowed"                         allow 'gh label list'
bash_case "G68 gh api with a write flag is blocked"              deny  'gh api repos/x/y -X DELETE' "write requests are denied"
bash_case "G69 gh api read is allowed"                           allow 'gh api repos/x/y/dependabot/alerts'
bash_case "G70 gh release create is blocked"                     deny  'gh release create v1.0' "is denied"
bash_case_at "$FIXTURE_FEAT" "G71 merging the protected base into a feature branch is allowed" allow 'git merge origin/main'
bash_case_at "$FIXTURE_FEAT" "G72 merging the bare base name is allowed"          allow 'git merge main'
bash_case_at "$FIXTURE_MAIN" "G73 merging into a protected branch is blocked"     deny  'git merge feat/test-branch' "landing into it"
bash_case_at "$FIXTURE_FEAT" "G74 merging a non-base branch is blocked"           deny  'git merge some-other-branch' "protected base branch"
bash_case_at "$FIXTURE_FEAT" "G75 merge -X ours is blocked"                       deny  'git merge origin/main -X ours' "without a visible conflict"
bash_case_at "$FIXTURE_FEAT" "G76 merge --abort is allowed"                       allow 'git merge --abort'
bash_case "G77 git rebase is blocked"                            deny  'git rebase main' "changes repository state"
bash_case "G78 git pull without --ff-only is blocked"            deny  'git pull origin main' "only with --ff-only"
bash_case "G78b git pull --ff-only is allowed"                   allow 'git pull --ff-only origin main'

# ---  PUBLISH_ENABLED=0 restores the pre-publishing blanket deny  ---
# Env-var driven, so this flips the running hook directly rather than
# patching a copy on disk.
off_case() {
  local label="$1" want="$2" command="$3" must_contain="${4:-}"
  local payload out got reason problem=""
  payload="$(printf '%s' "$command" | python3 -c 'import json,sys; print(json.dumps({"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":sys.stdin.read()}}))')"
  out="$(printf '%s' "$payload" | PUBLISH_ENABLED=0 python3 "$HOOK" 2>/dev/null)"
  got="$(decision_of "$out")"
  reason="$(reason_of "$out")"
  [ "$got" != "$want" ] && problem="decision=$got want=$want"
  if [ -z "$problem" ] && [ -n "$must_contain" ] && [[ "$reason" != *"$must_contain"* ]]; then
    problem="reason missing: $must_contain"
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

off_case "G79 off: push is blocked"                  deny  'git push -u origin feat/x'     "changes repository state"
off_case "G80 off: commit is blocked"                deny  'git commit -m x'               "changes repository state"
off_case "G81 off: switch -c is blocked"             deny  'git switch -c feat/x'          "changes repository state"
off_case "G82 off: branch creation is blocked"       deny  'git branch feat/x'             "creates a branch"
off_case "G83 off: stash push is blocked"            deny  'git stash'                     "changes repository state"
off_case "G84 off: merge --ff-only is still blocked" deny  'git merge --ff-only origin/main' "changes repository state"
off_case "G85 off: git add is blocked"               deny  'git add .'                     "changes repository state"
off_case "G86 off: read-only git still works"        allow 'git log --oneline -5'
off_case "G87 off: gh pr merge stays blocked"        deny  'gh pr merge 12'                "is denied"

# ──────────────────────────────  auto format  ──────────────────────────────
HOOK="$HOOKS/auto_format.py"
printf '\nauto format\n\n'

auto_format_case() {
  local label="$1" want_changed="$2" file="$3" tool="$4" path_override="${5:-}"
  local before after
  before="$(cat "$file")"
  if [ -n "$path_override" ]; then
    PATH="$path_override" printf '{"tool_name":"%s","tool_input":{"file_path":"%s"}}' "$tool" "$file" \
      | PATH="$path_override" python3 "$HOOK" > /dev/null 2>&1
  else
    printf '{"tool_name":"%s","tool_input":{"file_path":"%s"}}' "$tool" "$file" \
      | python3 "$HOOK" > /dev/null 2>&1
  fi
  local rc=$?
  after="$(cat "$file")"
  local problem=""
  if [ "$rc" -ne 0 ]; then
    problem="hook exited $rc instead of 0"
  elif [ "$want_changed" = "yes" ] && [ "$before" = "$after" ]; then
    problem="file was not reformatted"
  elif [ "$want_changed" = "no" ] && [ "$before" != "$after" ]; then
    problem="file was reformatted when it should have been left alone"
  fi
  if [ -z "$problem" ]; then
    printf '  PASS  %s\n' "$label"
    printf 'PASS\n' >> "$RESULTS"
  else
    printf '  FAIL  %s\n        %s\n' "$label" "$problem"
    printf 'FAIL\n' >> "$RESULTS"
  fi
}

FMT="$WORKDIR/fmt"
EMPTYBIN="$WORKDIR/emptybin"
mkdir -p "$FMT" "$EMPTYBIN"
ln -s "$(command -v python3)" "$EMPTYBIN/python3"

printf 'package main\n\nfunc  main() {}\n' > "$FMT/main.go"
auto_format_case "F1  gofmt reformats a .go file when gofmt is present" \
  yes "$FMT/main.go" "Write"

printf 'package main\n\nfunc  main() {}\n' > "$FMT/nogofmt.go"
auto_format_case "F2  no-ops silently when gofmt isn't on PATH" \
  no "$FMT/nogofmt.go" "Write" "$EMPTYBIN"

printf 'const   x = 1;\n' > "$FMT/widget.ts"
auto_format_case "F3  no-ops silently when prettier isn't on PATH" \
  no "$FMT/widget.ts" "Edit" "$EMPTYBIN"

printf 'not a real config file\n' > "$FMT/notes.txt"
auto_format_case "F4  an extension with no formatter is left alone" \
  no "$FMT/notes.txt" "Write"

expect "F5  MultiEdit is not in scope and never blocks" allow <<JSON
{"tool_name":"MultiEdit","tool_input":{"file_path":"$FMT/main.go"}}
JSON

expect "F6  a missing file_path never blocks" allow <<'JSON'
{"tool_name":"Write","tool_input":{}}
JSON

expect "F7  a malformed payload fails open, not closed" allow <<'JSON'
{"tool_name":"Write","tool_input":
JSON

# --- Context injector ---
printf '\ncontext injector\n\n'

inject_case() {
  local label="$1" root="$2" must_contain="${3:-}" must_not_contain="${4:-}"
  local out problem=""
  out="$(INJECT_ROOT="$root" python3 -c '
import os, sys, io
sys.path.insert(0, os.environ["HOOKS"])
import context_injector as ci
ci.repo_root = lambda start: os.environ["INJECT_ROOT"] or None
buf = io.StringIO()
sys.stdout = buf
try:
    ci.main()
except SystemExit:
    pass
except BaseException:
    sys.stdout = sys.__stdout__
    print("CRASHED")
    raise SystemExit(0)
sys.stdout = sys.__stdout__
print(buf.getvalue(), end="")
' 2>/dev/null)"

  if [[ "$out" == *CRASHED* ]]; then
    problem="hook crashed instead of failing open"
  fi
  if [ -z "$problem" ] && [ -n "$must_contain" ] && [[ "$out" != *"$must_contain"* ]]; then
    problem="output missing: $must_contain"
  fi
  if [ -z "$problem" ] && [ -n "$must_not_contain" ] && [[ "$out" == *"$must_not_contain"* ]]; then
    problem="output leaked: $must_not_contain"
  fi

  if [ -z "$problem" ]; then
    printf '  PASS  %s\n' "$label"
    printf 'PASS\n' >> "$RESULTS"
  else
    printf '  FAIL  %s\n        %s\n' "$label" "$problem"
    [ -n "$out" ] && printf '%s\n' "$out" | sed 's/^/        | /'
    printf 'FAIL\n' >> "$RESULTS"
  fi
}

export HOOKS

FIX="$WORKDIR/inject"
mkdir -p "$FIX/empty" "$FIX/full" "$FIX/nested/docs" "$FIX/junk"

printf '%s\n' '# Backlog' '## Now — V1' '### Create + fetch' \
  '- [C][WIP] Idempotent POST /orders · M · S2 → `go test ./orders/...`' \
  '- [H] POST /orders/:id/refund · M · S4 → `go test ./refund/...`' \
  '- [H][BLOCKED] Refund idempotency · M · S4 → `go test ./refund/...`' \
  '  - Blocked: the SDK exposes no idempotency key at the pinned version.' \
  > "$FIX/full/BACKLOG.md"

printf '%s\n' '# Changelog' '## [Unreleased]' '## [0.4.2] — 2026-08-20' \
  '### Added' '- Something.' > "$FIX/full/CHANGELOG.md"

printf '%s\n' '# Backlog' '- [M] a nested story · S → `true`' \
  > "$FIX/nested/docs/BACKLOG.md"

printf '%s\n' 'not a backlog at all' > "$FIX/junk/BACKLOG.md"

inject_case "I1  reports the WIP story"            "$FIX/full" "Idempotent POST /orders"
inject_case "I2  counts blocked stories"           "$FIX/full" "3 open, 1 BLOCKED"
inject_case "I3  reports the released version"     "$FIX/full" "Released version: 0.4.2"
inject_case "I4  skips the Unreleased heading"     "$FIX/full" "" "version: Unreleased"
inject_case "I5  an indented note is not a story"  "$FIX/full" "" "Blocked: the SDK"
inject_case "I6  no verify target is stated"       "$FIX/full" "Verify gate: none declared"
inject_case "I7  silent when no backlog exists"    "$FIX/empty" "" "Work state"
inject_case "I8  finds a backlog under docs/"      "$FIX/nested" "docs/BACKLOG.md"
inject_case "I9  a backlog with no stories is fine" "$FIX/junk" "Nothing marked [WIP]"
inject_case "I10 no version line without a changelog" "$FIX/nested" "" "Released version"
inject_case "I11 outside a repo it stays silent"   "" "" "Work state"
inject_case "I12 a missing root does not crash"    "/nonexistent/repo" "" "Work state"

PASS="$(grep -c '^PASS$' "$RESULTS" || true)"
FAIL="$(grep -c '^FAIL$' "$RESULTS" || true)"
printf '\n  %d passed, %d failed\n\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
