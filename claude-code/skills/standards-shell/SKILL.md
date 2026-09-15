---
name: standards-shell
description: Shell/bash standards — set -euo pipefail, quoting, shellcheck, function and variable naming. Load before reading or writing any .sh file or a shebang'd script with no extension.
user-invocable: false
paths: "**/*.sh"
---

# Shell

Zero comments below the header block. Error messages lead with a verb phrase and never name the function.

## Comment discipline

You must strictly limit code comments. **A non-compliant comment prompts the user for approval before the write lands — it does not fail outright.** That is deliberate while these directives are still being tuned: write only a comment you actually believe is warranted, since every miss costs the user a decision. **Deleting a comment you did not add always prompts too**, regardless of shape — moving or refactoring code is not licence to drop someone else's note.

Banned: a name echo (the comment's first word repeats the function/variable name below it); an implementation narrative (explaining *what* code is doing, or describing standard syntax).

Allowed only: a compiler/linter directive (always allowed); a step marker (indented, inside a function body, <= 80 chars); a banner/section break (<= 40-char label); an intent/WHY comment using the `WHY:`, `NOTE:`, or `TODO(author/issue):` prefix; a script manual — line comments directly under a `#!` shebang, this file's only place for a contiguous block.

This language's exempt machine directives, verified against the guard's own list: `shellcheck disable=`. The "Script manual" slot — line comments directly under a `#!` shebang — is this language's usual home for a run/exit-code summary; see `scripts/hooks/git/` in this repo for the lived shape. Anything past that header block prompts for approval.

## Toolchain

- **No version floor to declare** — POSIX-adjacent bash 3.2+ unless a script itself gates on a newer feature (associative arrays, `mapfile`) with a version check. State the floor in the script's own header comment when it matters.
- **`shellcheck` is the lint tool**, run against every `.sh` file; there is no accepted alternative. A suppression is `# shellcheck disable=<code>` directly above the flagged line, never a blanket disable for the whole file.
- **No standard formatter** — `shfmt` is optional, not required by anything in this repo. Match the indentation (two spaces) and brace style already in `scripts/hooks/git/`.
- **No internal dependency graph command** — nothing in the shell toolchain lists one. A script's internal edges are its `source`/`.` lines and the siblings it execs; read those directly. `assess-change-risk` states when that read is required and what an unmeasured fan-out does to the tier.
- **No dependency manager** — a shell script's "SDK" is the coreutils and the other scripts already in the repo. Read a sibling script before shelling out to a new external tool.

## Conventions

- **Write it in Python, not shell, when the script has to run on a platform without a POSIX shell.** A repo that claims Windows support cannot reach a `.sh` file from a native `cmd`/PowerShell session, from a hook whose command is a bare path, or from a CI runner with no shell image — so anything a developer or a gate invokes directly (a build script, a test harness, an installer, a verify entry point) is Python with a stdlib-only floor. Shell stays for what a shell already dispatches: a `core.hooksPath` git hook, which Git runs through its own bundled shell on every platform, and glue another tool invokes as `sh -c`. **Decide by who invokes it, not by how long it is** — a 20-line installer people run by hand is Python; a 200-line hook sibling git dispatches is shell.
- **A repo's verify gate must be invocable on every platform the repo claims to support**, by the same command, with no per-OS branch in the instructions. Check it against the platform list the README or the install doc actually states: an executable bit that only exists on a POSIX checkout, a `make`/`task`/`just` binary that is not a default install, and a `#!` line the OS does not honour each break that. Prefer an entry point invoked as `<interpreter> <path>`; state the gate's command once in `AGENTS.md` and have every other caller — a git hook, a CI job, an agent's Stop gate — resolve that same one.
- **`set -euo pipefail` at the top of every script** that isn't a sourced fragment, right after the shebang and header comment. Drop the `-e` only where the script must survive a failing step to tally or report results, and say so in the header comment.
- **Quote every expansion**: `"$var"`, `"$@"`, `"${arr[@]}"`. An unquoted expansion is a bug unless the line is deliberately doing word-splitting, which itself gets a `# shellcheck disable=SC2086` immediately above it.
- **`[ ]` (POSIX test) or `[[ ]]` (bash test), never `[ $x == $y ]` unquoted.** This repo's scripts use both; match whichever the file already uses rather than mixing.
- **Naming**: `lower_snake_case` for functions and local variables, `SCREAMING_SNAKE` for exported/global constants (`REPO_ROOT`, `TARGET`, `DRY_RUN`). A function that only prints takes a verb name (`say`, `usage`).
- **`local` every function-scoped variable** in a function that isn't just a one-line wrapper around a single command.
- **Prefer `printf` over `echo`** for anything with a variable in it — `echo` interprets backslash escapes inconsistently across shells; `printf '%s\n' "$msg"` does not.
- **Resolve the script's own directory** with `"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` rather than assuming the caller's `cwd` — every shell script in this repo does this.
- **Exit codes are deliberate**: `0` success, `1` a check failed, `2` a usage/argument error. A `usage()` + `exit 2` on a bad flag is the pattern to match.
- **A trap for cleanup**, not a manual `rm` at every exit path — a script that makes a temp directory removes it from a single `EXIT` trap.

## Security standards

Language-specific insecure-usage patterns for `/assess-security` to pull from, beyond `standards-api-security`'s generic OWASP checklist.

- **An unquoted variable reaching `eval`, a command substitution, or a subshell is command injection** — the quoting rule above is a correctness rule and a security control at once; there is no separate "sanitize for security" pass.
- **`curl <url> | bash` (or `| sh`) executes an unreviewed remote script with the invoking user's privileges** — download, verify (checksum or signature), then run, never pipe straight into a shell.
- **A predictable temp path (`/tmp/$$`, `/tmp/myapp-$RANDOM`) is a symlink/race target.** Use `mktemp`/`mktemp -d` so the name and the file's existence are both attacker-unpredictable.
- **`set -x`/`bash -x` echoes every expanded command, secrets included, to whatever captures stdout/stderr** — never enable it in a script that ever sees a credential or token.

## Testing

- **No `bats` or other shell test framework is in use** — this repo's shell surface (the git-hook dispatchers and their installer; everything under `claude-code/hooks/` and `scripts/*.py` is Python) is covered by hand-rolled stdlib harnesses that invoke the script under test as a subprocess and assert on its exit code and output, no external test runner.
- **Follow that shape for a new shell script under test**: a sibling `test_<name>.py` that runs the script against fixture input/args and asserts exit code and stdout/stderr by `difflib` or a string match, no framework dependency added.
- **`python3 scripts/validate.py` and `python3 scripts/test_hooks.py` together are this repo's shell-adjacent verify surface** — `scripts/verify.py`, the portable entry point `make verify` wraps, runs both. Tier definitions, merge/release gates, and coverage floors beyond that are owned by `standards-sdlc`.

## Quick-reference fields

The field set a shell-heavy repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Version floor | the script's own header comment, if it states one |
| Lint | presence of a `.shellcheckrc` or CI invocation of `shellcheck` |
| Entrypoint(s) | the executable `.sh` files at the repo root or `scripts/` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for shell** in `git-repo-init` — a shell script is glue around another language's repo, never a repo type of its own, so no repo type token maps here.

## Reference material

- **`scripts/hooks/git/install.sh` and its `pre-commit`/`pre-push` dispatchers** in this repo — the lived example for header-comment shape, `set -euo pipefail`, and the quoting this skill describes.
