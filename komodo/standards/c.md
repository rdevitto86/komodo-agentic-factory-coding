# C

Follow the comments standard: a doc line on every public function, a comment on a private one only when long or non-obvious.

**Hosted, application-level C.** Firmware, ISR discipline, RTOS, and bare-metal constraints are the cpp standard's domain — load that instead for embedded work; this standard assumes a normal OS process.

## Conventions

- **Match the codebase's existing naming.** C has no single dominant convention the way Go or Python does; `snake_case` is common but a project already committed to something else wins.
- **One header per translation unit's public surface.** The `.h` declares, the `.c` defines. Include guards as `#pragma once` unless the codebase already uses the `#ifndef`/`#define` form.
- **`const`-correct by default** — a pointer parameter the function doesn't mutate through is `const`.
- **No global mutable state** without a name that says so; justify it in `BACKLOG.md`, never in an inline comment.
- **Minimise a header's own includes.** Forward-declare where a full definition isn't needed; push includes into the `.c` file.
- **Depend on a function-pointer/vtable-style seam the caller owns, not a concrete implementation** — the Dependency Inversion Principle applied to C. A consumer takes a struct of function pointers (or a single callback) that it declares the shape of; the concrete implementation is wired in by the caller, never hard-linked into the consumer.
- **Wrap a long statement by collapsing one level at a time, never straight to one-arg-per-line.** Try the whole statement on one line first; if it doesn't fit, move the wrapped content to its own indented line(s) with a trailing comma and the closing paren/brace on its own line at the original indent — a function call may stay grouped at this step if it fits, but a struct-literal's fields never do, always one field per line once wrapped. Only if that grouped form is still too long, break to one item per line.
- **A numeric or short repeated string literal standing for a size, TTL, count, threshold, or spec-level value (a header name, a status string) gets extracted to a named `#define` or `static const`.** File-scope when shared by more than one function in the translation unit; a function-local `const` when scoped to one call. A literal repeated across 2+ files or functions is the strongest signal to extract first. Two or more constants introduced together are declared together, adjacent, not scattered across the file.
- **No blank line between two consecutive early-return guard clauses.** Exactly one blank line between the last guard clause in a sequence and the happy-path logic that follows it.

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

## Security standards

Language-specific insecure-usage patterns beyond the api-security standard.

- **`system()`/`popen()` with any input derived from outside the process is command injection** — a shell interprets the string, so validation upstream does not close it.
- **A format string must never be attacker-influenced.** `printf(user_input)` reads/writes memory through `%n`/`%s` in the input; always `printf("%s", user_input)`.
- **`strcpy`/`strcat`/`sprintf`/`gets` are exploitable buffer overflows, not just crash bugs** — the Memory and resources section above states the bounded-function replacement; this is the same rule read as a security control, not a stability one.
- **An integer overflow feeding an allocation size is a heap-overflow primitive** — check the multiplication/addition against `SIZE_MAX` before it reaches `malloc`/`calloc`.
- **`rand()`/`srand()` are not a CSPRNG.** A token, key, or nonce needs `/dev/urandom`, `getrandom(2)`, or the platform's crypto library.

## Testing

- **Unity or CMocka, whichever the project already uses** — Unity for pure unit tests, CMocka when a syscall or library boundary needs mocking.
- **Unit tests colocate** as `<name>_test.c` beside the file they cover, built as a separate target.
- **Sanitizer-enabled test builds are a separate CI target**, not the default build — the cost is real and paid once per run, not per developer build.
- **Coverage floors and merge gates are owned by the sdlc standard**; this section covers C mechanics only.
