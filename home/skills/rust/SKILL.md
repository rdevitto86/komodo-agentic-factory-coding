---
name: rust
description: Rust standards — errors, ownership at boundaries, async, unsafe policy. Load before reading or writing any .rs or Cargo.toml file.
user-invocable: false
---

# Rust

Zero comments, zero doc comments (`///`, `//!`). Error strings lead with a verb phrase and never name the function.

## Tooling

- **`cargo fmt` and `cargo clippy -- -D warnings`** both pass before anything is considered done.
- **Edition 2021 or later**, MSRV pinned in `Cargo.toml`.
- **A suppression is a bare `#[allow(...)]`** with the reason recorded in `TODO.md`, never in a comment. Prefer fixing the lint.

## Errors

- **`thiserror` for libraries, `anyhow` for binaries.** Never both in the same crate's public API.
- **Never `unwrap()` or `expect()` outside tests and `main`.** In `main`, an `expect` message is a verb phrase describing what failed.
- **`?` over manual matching** when the conversion is already defined.
- **Wrap with context** at each boundary: `.with_context(|| "failed to read widget")`.
- **`panic!` is for programmer error only** — never for control flow, never across a public API.

## Types and ownership

- **Newtypes carry domain meaning** — `struct UserId(String)`. The compiler catches argument-order bugs for free.
- **Make illegal states unrepresentable.** An enum with data beats a struct of `Option` fields that are only valid in combination.
- **Parse at the edge.** `serde` into a DTO, convert once into the domain type, and let the core accept only the domain type.
- **Accept `&str` / `&[T]`, return owned.** Do not force a caller to allocate to call you.
- **Fallible construction returns `Result<Self, E>`** — never a constructor plus a separate `validate()`.
- **Traits stay small and live at the consumer.** Do not publish a trait beside its only implementation.

## Async

- **Tokio unless there is a reason otherwise**, and only one runtime per binary.
- **Never block in an async context.** CPU-bound work goes to `spawn_blocking` or a dedicated pool.
- **Every await that crosses a network boundary gets a timeout.**
- **Own every spawned task.** A detached `tokio::spawn` with no join handle is a leak; use `JoinSet` for fan-out so the first failure can cancel its siblings.
- **Bound every channel.** An unbounded channel is an unbounded memory leak under load.

## Unsafe

- **`#![forbid(unsafe_code)]` at the crate root by default.**
- **Removing it requires the user's explicit sign-off**, and the invariant being upheld goes in `TODO.md`, not a comment.

## Testing

- **Unit tests colocate** in a `#[cfg(test)] mod tests` block.
- **Integration tests live in `tests/`**, flat by feature.
- **`proptest` for parsers and codecs** where the input space is large.
- **`cargo test` runs clean under `--all-features`.**
- **Tier definitions, merge/release gates, and coverage floors are owned by the `sdlc` skill** (90% minimum on new code, 100% on SDKs/shared libraries and security-critical paths) — this section covers Rust mechanics only.
