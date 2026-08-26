---
name: standards-shell
description: Shell/bash standards — set -euo pipefail, quoting, shellcheck, function and variable naming. Load before reading or writing any .sh file or a shebang'd script with no extension.
user-invocable: false
paths: "**/*.sh"
---

# Shell

Zero comments below the header block. Error messages lead with a verb phrase and never name the function.

## Comment discipline

`rules-commenting` carries the shared template contract. This language's exempt machine directives, verified against the guard's own list: `shellcheck disable=`. The "Script manual" slot — line comments directly under a `#!` shebang — is this language's usual home for a run/exit-code summary; see `setup.sh` and `scripts/*.sh` in this repo for the lived shape. Anything past that header block prompts for approval.

## Toolchain

- **No version floor to declare** — POSIX-adjacent bash 3.2+ unless a script itself gates on a newer feature (associative arrays, `mapfile`) with a version check. State the floor in the script's own header comment when it matters.
- **`shellcheck` is the lint tool**, run against every `.sh` file; there is no accepted alternative. A suppression is `# shellcheck disable=<code>` directly above the flagged line, never a blanket disable for the whole file.
- **No standard formatter** — `shfmt` is optional, not required by anything in this repo. Match the indentation (two spaces) and brace style already in `scripts/*.sh` and `setup.sh`.
- **No dependency manager** — a shell script's "SDK" is the coreutils and the other scripts already in the repo. Read a sibling script before shelling out to a new external tool.

## Conventions

- **`set -euo pipefail` at the top of every script** that isn't a sourced fragment, right after the shebang and header comment. `test-hooks.sh` uses `set -uo pipefail` instead because it must survive a failing case and tally results — the deliberate exception, not a template.
- **Quote every expansion**: `"$var"`, `"$@"`, `"${arr[@]}"`. An unquoted expansion is a bug unless the line is deliberately doing word-splitting, which itself gets a `# shellcheck disable=SC2086` immediately above it.
- **`[ ]` (POSIX test) or `[[ ]]` (bash test), never `[ $x == $y ]` unquoted.** This repo's scripts use both; match whichever the file already uses rather than mixing.
- **Naming**: `lower_snake_case` for functions and local variables, `SCREAMING_SNAKE` for exported/global constants (`REPO_ROOT`, `TARGET`, `DRY_RUN`). A function that only prints takes a verb name (`say`, `usage`) the way `setup.sh` does.
- **`local` every function-scoped variable** in a function that isn't just a one-line wrapper around a single command.
- **Prefer `printf` over `echo`** for anything with a variable in it — `echo` interprets backslash escapes inconsistently across shells; `printf '%s\n' "$msg"` does not.
- **Resolve the script's own directory** with `"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` rather than assuming the caller's `cwd` — every script in `scripts/` and `setup.sh` does this.
- **Exit codes are deliberate**: `0` success, `1` a check failed, `2` a usage/argument error. `setup.sh`'s `usage()` + `exit 2` on a bad flag is the pattern to match.
- **A trap for cleanup**, not a manual `rm` at every exit path — `test-hooks.sh`'s `trap 'rm -rf "$WORKDIR"; ...' EXIT` is the reference.

## Testing

- **No `bats` or other shell test framework is in use** — this repo's own bash surface (`claude-code/hooks/*.py` are Python; `scripts/*.sh` and `setup.sh` are the actual shell) is tested by a hand-rolled harness, `scripts/test-hooks.sh`: a plain bash script that feeds each hook a JSON payload on stdin and asserts the returned decision, no external test runner.
- **Follow that shape for a new shell script under test**: a sibling `test-<name>.sh` that runs the script against fixture input/args and asserts exit code and stdout/stderr with `diff` or a string match, no framework dependency added.
- **`bash scripts/validate.sh` and `bash scripts/test-hooks.sh` together are this repo's shell-adjacent verify surface** — `.claude/verify.sh` runs both. Tier definitions, merge/release gates, and coverage floors beyond that are owned by `standards-sdlc`.

## Quick-reference fields

The field set a shell-heavy repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Version floor | the script's own header comment, if it states one |
| Lint | presence of a `.shellcheckrc` or CI invocation of `shellcheck` |
| Entrypoint(s) | the executable `.sh` files at the repo root or `scripts/` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for shell** in `generate-repo` — a shell script is glue around another language's repo, never a repo type of its own, so no repo type token maps here.

## Reference material

- **`scripts/setup.sh`, `scripts/test-hooks.sh`, `scripts/validate.sh`, `scripts/release.sh`** in this repo — the lived example for header-comment shape, `set -euo pipefail`, quoting, and the trap-based cleanup pattern this skill describes.
