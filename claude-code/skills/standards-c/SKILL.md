---
name: standards-c
description: C standards — toolchain, memory and resource discipline, error handling, testing. Load before reading or writing any .c or .h file. Hosted/application C only — embedded and firmware constraints belong to `standards-cpp`.
user-invocable: false
paths: "**/*.c, **/*.h"
---

# C

Zero comments, zero header doc-blocks. Errors lead with a verb phrase and never name the function.

**Hosted, application-level C.** Firmware, ISR discipline, RTOS, and bare-metal constraints are `standards-cpp`'s domain — load that instead for embedded work; this skill assumes a normal OS process.

## Comment discipline

`rules-commenting` carries the shared template contract. This language's exempt machine directives, verified against the guard's own list: `// clang-format off`, `// clang-format on`, `// NOLINT`, `// NOLINTNEXTLINE` (matches the `nolint` prefix). Anything else — including a header-comment block above a function — prompts for approval.

## Toolchain

- **The C standard is whatever the build declares** — `-std=` in the Makefile, or `CMAKE_C_STANDARD` in `CMakeLists.txt`. Read it; never assume C99 vs C11 vs C17.
- **`clang-format`** for formatting, config at `.clang-format`. **`clang-tidy`** and/or **`cppcheck`** as the static-analysis gate.
- **Sanitizers over guesswork** — AddressSanitizer and UndefinedBehaviorSanitizer (`-fsanitize=address,undefined`) on every debug build; ThreadSanitizer for anything touching shared state across threads. A memory bug a sanitizer would have caught in CI is a process failure, not bad luck.
- **`valgrind --leak-check=full`** where sanitizers aren't wired into the build yet.

## Conventions

- **Match the codebase's existing naming.** C has no single dominant convention the way Go or Python does; `snake_case` is common but a project already committed to something else wins.
- **One header per translation unit's public surface.** The `.h` declares, the `.c` defines. Include guards as `#pragma once` unless the codebase already uses the `#ifndef`/`#define` form.
- **`const`-correct by default** — a pointer parameter the function doesn't mutate through is `const`.
- **No global mutable state** without a name that says so; justify it in `BACKLOG.md`, never in an inline comment.
- **Minimise a header's own includes.** Forward-declare where a full definition isn't needed; push includes into the `.c` file.

## Memory and resources

- **Check every allocation.** `malloc`/`calloc`/`realloc` can return `NULL`; an unchecked pointer dereferenced past that point is a crash with no diagnostic.
- **One free, one owner.** Whichever function allocates documents ownership through its return convention, not a comment — a `create_x`/`destroy_x` pair, matched 1:1.
- **Null a pointer after freeing it** wherever the same variable could plausibly be freed twice or used after.
- **Bounded string functions only** — `snprintf`, `strlcpy`/`strlcat`, or their explicit-length equivalents. `strcpy`, `strcat`, `sprintf`, and `gets` are never acceptable, sanitizer or not.
- **No VLAs in anything shipping.** A stack array sized by input data is an unbounded stack allocation.
- **Every `fopen`/`malloc`/socket paired with a matching close/free/shutdown on every exit path**, including error returns — `goto cleanup` over duplicated teardown when a function has more than one failure point.

## Errors

- **Return codes or `errno`-style out-parameters, chosen once per project and applied everywhere** — never mix conventions within one codebase.
- **Never ignore a fallible return value.** A call whose failure is genuinely irrelevant still gets an explicit `(void)` cast, so the omission reads as a decision, not an oversight.
- **Errors propagate up; only the top of the call stack logs.** Log-and-return duplicates one root cause into multiple log lines.

## Testing

- **Unity or CMocka, whichever the project already uses** — Unity for pure unit tests, CMocka when a syscall or library boundary needs mocking.
- **Unit tests colocate** as `<name>_test.c` beside the file they cover, built as a separate target.
- **Sanitizer-enabled test builds are a separate CI target**, not the default build — the cost is real and paid once per run, not per developer build.
- **Coverage floors and merge gates are owned by the `standards-sdlc` skill**; this section covers C mechanics only.

## Quick-reference fields

The field set a C repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Standard | `-std=` in the Makefile, or `CMAKE_C_STANDARD` |
| Build system | `Makefile` or `CMakeLists.txt` presence |
| Compiler | Whatever the build invokes — read it, don't assume `gcc` |
| Entrypoint | The `main()` translation unit |

Drop a row whose value the repo genuinely lacks.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for C** in `write-repo` — no repo type token maps here. Use `write-repo`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here first.
