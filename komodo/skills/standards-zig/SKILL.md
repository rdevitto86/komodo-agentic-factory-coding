---
name: standards-zig
description: Zig: allocators, error unions, comptime, and the standard library first.
globs: ["**/*.zig", "**/*.zon", "**/build.zig"]
---

# Zig

Conventions for engineering in Zig. Principles live here; a version number never does — Zig's own compiler and standard library are still pre-1.0 and change release to release, so anything version-dependent is phrased relative to what the project's own `build.zig.zon` declares (its `minimum_zig_version` field, where present) rather than a hardcoded release. Read that manifest before assuming a language feature or standard-library API is available.

## Memory & allocators

- **Pass the allocator explicitly.** A function that allocates takes an `std.mem.Allocator` parameter; it never reaches for a hidden global or package-level allocator. The caller decides the allocation strategy, not the callee — this is what makes the same code work under a general-purpose allocator in production and a leak-detecting test allocator in `zig test`.
- **Reach for an arena when the lifetime is short-lived and bounded** — a request handler, a single parse pass, a compiler invocation. Allocate freely into an `std.heap.ArenaAllocator` and free everything at once with one `deinit()`, rather than tracking each allocation's lifetime individually. Don't reach for an arena when the data must outlive the scope that created it, or when the workload is long-running and the arena would grow unbounded.
- **`defer` releases in the same scope that acquires; `errdefer` releases only on the error path.** A resource acquired and an error union returned in the same function pairs an `errdefer` right after acquisition, so a partially-constructed value is cleaned up exactly when construction fails and left alone when it succeeds.
- **Zig has no hidden control flow, and allocator discipline is what that promise depends on.** No operator overloading, no hidden allocation behind a language construct, no hidden exception unwinding — a call that can allocate or fail is visible at the call site (`Allocator` parameter, `!T` return). Preserve that visibility in application code the same way the language does: an allocating function stays honest about it in its signature rather than hiding an allocator behind a struct field a caller can't see.

## Error handling

- **Error unions (`!T`) and error sets are the idiomatic propagation mechanism** — a fallible function returns `SomeError!T` (or an inferred error set) instead of a sentinel value, an `errno`-style out-parameter, or a boolean plus a side-channel.
- **`try` propagates, `catch` handles.** Use `try expr` to bubble a fallible call's error up to the caller unchanged; use `catch` only at the point that actually has a recovery action (a fallback value, a retry, a log-and-continue) — a bare `catch unreachable` asserts the error path is provably impossible, not a way to silence a case that hasn't been handled.
- **Prefer a named error set once more than one function shares it, or once the set is part of a public API.** An inferred error set (the return type omitting the set name before `!T`) is fine for a small, private helper; a function other code depends on for exhaustive `switch`-on-error handling, or one whose error set several callers need to reference by name, earns an explicit `const SomeError = error{...}`.
- **Never swallow an error silently.** A caught error gets logged, converted to a different error the caller can act on, or explicitly and visibly discarded with a comment explaining why that path is safe — never a bare `catch {}` that drops the failure on the floor.

## Comptime

- **Reach for `comptime` when the value or the type shape is genuinely known at compile time** — a generic container or algorithm parameterized over a type, a fixed-size buffer whose bound is a compile-time constant, build-time configuration baked into the binary. This is what replaces generics, templates, and much of what other languages need macros for.
- **A `comptime` parameter expresses a generic through the type system, not a runtime dispatch table.** `fn max(comptime T: type, a: T, b: T) T` is a compile-time-specialized function per `T`; prefer this over a runtime type tag plus a switch, or an `anyopaque` plus manual casting, whenever the set of types is known at the call site.
- **Don't reach for `comptime` where a plain runtime value would do.** Compile-time evaluation adds real complexity — longer compile times, harder-to-read error traces through generic instantiations, code that only makes sense once you trace which values are compile-time-known. A value that varies per request or per user is a runtime value; forcing it through `comptime` to "be generic" just because the language allows it is needless.

## Build system

- **`build.zig` and `build.zig.zon` are the toolchain and dependency manifest** — `build.zig` declares the build graph (targets, steps, artifacts), `build.zig.zon` declares the package's own identity, its dependencies, and the version floor. There is no separate build-config DSL to reach for; the build graph is Zig code, written and read the same way application code is.
- **Pin dependencies through the manifest, not an external lockfile-style tool.** `build.zig.zon` records each dependency's location and content hash directly; that hash is the reproducibility guarantee — a build that resolves a dependency to content matching the recorded hash is reproducible by construction, with no separate lockfile format to keep in sync.
- **A reproducible build follows from an accurate manifest, not from pinning the compiler by hand outside it.** Where the project needs a version floor recorded, that floor belongs in `build.zig.zon`'s own field for it — read that field rather than a comment or a README claim before assuming what's required.

## Testing

- **`zig test` and `std.testing` are the test stack** — `std.testing.expect`, `std.testing.expectEqual`, and friends inside `test "descriptive name" { ... }` blocks. There is no separate assertion library to reach for by default.
- **Tests live colocated with the code they test**, per Zig convention — a `test` block in the same file as the function it exercises, not split into a parallel test-tree mirroring the source layout. This keeps a change to a function and its test in one diff hunk.
- **`std.testing.allocator` catches a leak by failing the test**, not by relying on a separate tool — pass it as the allocator under test wherever the code under test takes one, so an unfreed allocation surfaces as a test failure at the point that introduced it.

## C interop

- **`@cImport`/`@cInclude` bring a C header's declarations into Zig directly** — translate-c handles the header at compile time, so there is no separate binding-generation step to run and commit ahead of the build.
- **Reach for C interop only when a real C dependency is unavoidable** — an existing C library with no practical pure-Zig equivalent, a platform API only exposed through a C header, or a performance-critical routine already implemented and validated in C. Don't pull in a C dependency for something a small amount of pure Zig would do just as well — a C dependency costs cross-compilation complexity (its own toolchain, its own platform-specific headers) that pure Zig code doesn't.
- **Where an FFI boundary is warranted, keep it narrow and wrapped** — a thin Zig layer around the `@cImport`'d declarations that converts C's raw pointers and sentinel-based error signaling into Zig's slices and error unions at the boundary, so the rest of the codebase never touches the C API's calling convention directly.
