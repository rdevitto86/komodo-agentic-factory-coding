---
name: standards-zig
description: Zig: allocators, error unions, comptime, and the standard library first.
globs: ["**/*.zig", "**/*.zon", "**/build.zig"]
---

# Zig

Principles live here; a version number never does — Zig's compiler and standard library are pre-1.0 and change release to release, so anything version-dependent is phrased relative to the project's `build.zig.zon` (`minimum_zig_version`, where present). Read that manifest before assuming a feature or API is available.

## Memory & allocators

- **Pass the allocator explicitly.** A function that allocates takes an `std.mem.Allocator` parameter, never a hidden global or package-level allocator. The caller decides the allocation strategy, not the callee, so the same code works under a general-purpose allocator in production and a leak-detecting one in `zig test`.
- **Reach for an arena when the lifetime is short-lived and bounded** — a request handler, a parse pass, a compiler invocation. Allocate freely into an `std.heap.ArenaAllocator` and free everything at once with `deinit()`, rather than tracking each allocation's lifetime. Skip an arena when data must outlive the creating scope, or the workload is long-running.
- **`defer` releases in the same scope that acquires; `errdefer` releases only on the error path.** A resource acquired and an error union returned pairs an `errdefer` right after acquisition, cleaning up a partial value on failure, left alone on success.
- **Zig has no hidden control flow, and allocator discipline is what that promise depends on.** No operator overloading, no hidden allocation, no hidden exception unwinding — a call that can allocate or fail is visible at the call site (`Allocator` parameter, `!T` return). An allocating function stays honest in its signature, not hidden behind a struct field.

## Error handling

- **Error unions (`!T`) and error sets are the idiomatic propagation mechanism** — a fallible function returns `SomeError!T` (or an inferred error set) instead of a sentinel value, an `errno`-style out-parameter, or a boolean plus side-channel.
- **`try` propagates, `catch` handles.** Use `try expr` to bubble an error up unchanged; use `catch` only at a point with a real recovery action (fallback value, retry, log-and-continue) — a bare `catch unreachable` asserts the error path is provably impossible, not a way to silence an unhandled case.
- **Prefer a named error set once more than one function shares it, or it's part of a public API.** An inferred error set (omitting the name before `!T`) is fine for a small, private helper; one callers rely on for exhaustive `switch`-on-error earns an explicit `const SomeError = error{...}`.
- **Never swallow an error silently.** A caught error gets logged, converted to one the caller can act on, or discarded with a comment explaining why it's safe — never a bare `catch {}` dropping the failure.

## Comptime

- **Reach for `comptime` when the value or type shape is known at compile time** — a generic container parameterized over a type, a fixed-size buffer with a compile-time bound, build-time configuration baked into the binary. This replaces generics, templates, and much of what other languages need macros for.
- **A `comptime` parameter expresses a generic through the type system, not a runtime dispatch table.** `fn max(comptime T: type, a: T, b: T) T` is compile-time-specialized per `T`; prefer this over a runtime type tag plus switch, or `anyopaque` plus manual casting, whenever types are known at the call site.
- **Don't reach for `comptime` where a plain runtime value would do.** Compile-time evaluation adds real complexity — longer compile times, harder-to-read error traces through generic instantiations. A value varying per request or user is a runtime value; forcing it through `comptime` to "be generic" is needless.

## Build system

- **`build.zig` and `build.zig.zon` are the toolchain and dependency manifest** — `build.zig` declares the build graph (targets, steps, artifacts), `build.zig.zon` declares the package's identity, dependencies, and version floor. There is no separate build-config DSL; the build graph is Zig code.
- **Pin dependencies through the manifest, not an external lockfile tool.** `build.zig.zon` records each dependency's location and content hash directly; a build matching the recorded hash is reproducible by construction, with no lockfile to keep in sync.
- **A reproducible build follows from an accurate manifest, not pinning the compiler by hand.** Where a version floor is needed, it belongs in `build.zig.zon`'s own field — read that field rather than a comment or README claim.

## Testing

- **`zig test` and `std.testing` are the test stack** — `std.testing.expect`, `std.testing.expectEqual`, and friends inside `test "descriptive name" { ... }` blocks, with no separate assertion library by default.
- **Tests live colocated with the code they test**, per Zig convention — a `test` block in the same file as the function it exercises, not a parallel test-tree. This keeps a function and its test in one diff hunk.
- **`std.testing.allocator` catches a leak by failing the test**, not a separate tool — pass it wherever the code under test takes an allocator, so an unfreed allocation surfaces at the point that introduced it.

## C interop

- **`@cImport`/`@cInclude` bring a C header's declarations into Zig directly** — translate-c handles the header at compile time, so there is no binding-generation step to run and commit.
- **Reach for C interop only when a real C dependency is unavoidable** — an existing C library with no pure-Zig equivalent, a platform API only exposed through a C header, or a performance-critical routine already validated in C. Don't pull one in for what a little pure Zig would do as well — it costs cross-compilation complexity pure Zig doesn't.
- **Where an FFI boundary is warranted, keep it narrow and wrapped** — a thin Zig layer around the `@cImport`'d declarations converting C's raw pointers and sentinel errors into Zig's slices and error unions, so the rest of the codebase never touches the C calling convention directly.
