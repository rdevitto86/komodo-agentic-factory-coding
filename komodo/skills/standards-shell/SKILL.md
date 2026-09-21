---
name: standards-shell
description: Shell: strict mode, quoting, exit codes, and when a script becomes a program.
globs: ["**/*.bash", "**/*.sh"]
---

# Shell

## When shell is the wrong tool
- Anything a developer or a gate invokes directly on a repo that claims Windows support is Python with a stdlib floor: build scripts, installers, verify entry points, test harnesses.
- Shell stays for what a shell already dispatches: a `core.hooksPath` git hook, which Git runs through its bundled shell everywhere, and glue another tool invokes as `sh -c`.
- A repo's verify gate is one command that resolves the same way on every platform it claims. Prefer `<interpreter> <path>` over an executable bit or a `make` binary.

## Comments
- A header block under the shebang says what the script does and how it is run. Below it, a comment only where a line cannot say it itself.
- A `# shellcheck disable=SCxxxx` directive sits directly above the line it covers.

## Conventions
- `set -euo pipefail` after the shebang and header. Drop `-e` only where the script must survive a failing step to report results, and say so in the header.
- Quote every expansion: `"$var"`, `"$@"`, `"${arr[@]}"`. Deliberate word-splitting carries a shellcheck directive.
- `[ ]` or `[[ ]]`, matching what the file already uses. Never an unquoted comparison.
- `lower_snake_case` functions and locals, `SCREAMING_SNAKE` globals. `local` every function-scoped variable.
- `printf '%s\n' "$msg"` over `echo` for anything holding a variable.
- Resolve the script's own directory with `"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`.
- Exit codes: `0` success, `1` a check failed, `2` a usage error, with a `usage()` on a bad flag.
- Cleanup in a single `EXIT` trap, never a manual remove at every exit path.

## Security
- An unquoted variable reaching `eval`, a command substitution, or a subshell is command injection.
- Never `curl <url> | sh`. Download, verify the checksum or signature, then run.
- `mktemp` or `mktemp -d` for every temp path; never `/tmp/$$` or `$RANDOM`.
- Never `set -x` in a script that can see a credential.

## Testing
- A shell script under test gets a sibling Python test that runs it as a subprocess and asserts on exit code and output. No shell test framework.
