---
name: standards-cpp
description: C++ for robotics and modules: RAII, ownership, real-time loops, and the standard library first.
globs: ["**/*.cc", "**/*.cpp", "**/*.hpp", "**/*.hh", "**/*.cxx"]
---

# C++

Komodo's default for robotics and for modules other code links against. A suggestion, not a mandate: an existing codebase keeps its language, and firmware follows the embedded standard, where Zig is the default.

Follow the comments standard: a doc line on every public function, a comment on a private one only when long or non-obvious.

## Confirm before writing

- **Standard and toolchain**: the C++ version the build pins, compiler, and sanitizers in CI. Read `CMakeLists.txt` or the build file first.
- **Framework**: ROS 2, a vendor SDK, or none. Follow its node, lifecycle, and executor model; never invent a parallel one.
- **Real-time budget**: which loops are hard real-time, their rates, and what may allocate or lock inside them.

## Ownership

- **RAII for every resource**: memory, files, sockets, locks, device handles. A raw `new` or `delete` outside an allocator is a finding.
- **`std::unique_ptr` by default**, `std::shared_ptr` only for shared lifetime a comment names. Raw pointers and references never own.
- **Value semantics first.** Pass small types by value, large ones by `const&`, sinks by value and move.
- **Rule of zero.** A class that owns nothing custom declares no destructor, copy, or move.

## Real-time loops

- **No allocation, locking, logging to disk, or exceptions** inside a hard real-time loop. Preallocate at configure time.
- **Lock-free single-producer queues** or a double buffer between the loop and everything else.
- **Time from a monotonic clock**, never wall time; a missed deadline is counted and reported, not ignored.

## Modules and interfaces

- **A stable public header per module**; implementation detail behind it. Nothing in a public header includes a heavy dependency it does not expose.
- **An ABI boundary** exported to other languages is `extern "C"` with plain types, and errors cross it as codes, never exceptions.
- **Errors**: `std::expected` or a result type on expected failures; exceptions only for the truly exceptional, and never across a module or real-time boundary.

## Tooling

- Build warnings as errors, `-Wall -Wextra -Wpedantic` or the compiler's equivalent.
- `clang-format` owns layout; `clang-tidy` and the sanitizers (address, undefined, thread) run in CI.
- Tests with the framework the repo already uses; a hardware-dependent test is marked and runs on the rig.

## Flag aggressively

- An owning raw pointer, or a `shared_ptr` cycle
- Allocation, a mutex, or I/O inside a real-time callback
- A data race: shared state across threads without an atomic or a lock
- An exception that can escape a callback, a thread, or a C boundary
- Undefined behaviour: signed overflow, out-of-bounds, use after move
