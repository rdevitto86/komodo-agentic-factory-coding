# Python Standards

Base Python standards for Komodo. Builds on `~/.claude/agents/swe/go/coding.md` and `~/.claude/agents/swe/ts/coding.md` — same engineering bar, language-specific idioms. Project configs may extend these but should not contradict.

---

## 1. Versions and tooling

- Python 3.12+ minimum. Pin the exact minor in `pyproject.toml` (`requires-python = ">=3.12,<3.13"`).
- Package and env management with `uv` (preferred) or `poetry`. Never raw `pip` in project workflows.
- Format with `ruff format`; lint with `ruff check`. One tool, no `black`/`isort`/`flake8`/`pylint` stack.
- Type-check with `mypy --strict` (or `pyright` in strict mode). Type errors block merge.
- Pre-commit hooks run `ruff`, `mypy`, and the test runner on staged files.

---

## 2. Type safety

- `from __future__ import annotations` at the top of every module — postponed evaluation, cheaper imports.
- Type-annotate **every** function signature and module-level binding. Local variables only when the inference is ambiguous.
- `Any` is forbidden without an inline `# type: ignore[reason]` justification.
- Use `typing.Protocol` (structural) for interfaces at boundaries; ABCs only when nominal subtyping is genuinely required.
- Runtime validation at all external boundaries with `pydantic` v2 or `attrs` — types alone are not runtime guarantees.
- Prefer `TypedDict` / `dataclasses` / `pydantic.BaseModel` over loose `dict[str, Any]`.

---

## 3. Error handling

- Catch specific exceptions, never bare `except:` or `except Exception:` without re-raise.
- Wrap and re-raise with context using a short descriptive phrase: `raise OrderError("failed to fetch order") from err`. Never the function name in the message — function context belongs in structured log fields.
  - Bad: `raise OrderError("get_order_by_id: db query failed") from err`
  - Good: `raise OrderError("failed to query order by ID") from err`
- Define module-level exception classes that inherit a single project base (`KomodoError`). Callers can `except KomodoError` once.
- Errors propagate up; log once at the boundary where you stop propagating.
- No `assert` for runtime checks — `assert` is stripped under `python -O`. Use explicit `raise`.

---

## 4. Naming

| Thing | Convention | Example |
|---|---|---|
| Modules, packages | lower_snake_case | `order_service.py` |
| Classes, type aliases | PascalCase | `OrderSummary`, `UserId` |
| Functions, methods, variables | snake_case | `fetch_order`, `is_loading` |
| Constants | SCREAMING_SNAKE | `MAX_RETRY_COUNT` |
| Private | leading underscore | `_internal_helper` |
| Type variables | PascalCase, short | `T`, `OrderT` |

- Doc comments (docstrings) must not open with the function name. Write `"""Return the order for the given ID."""` not `"""fetch_order returns the order ..."""`.
- Boolean names prefix `is_`, `has_`, `can_`, `should_`.
- HTTP variables: `req` for request, `res` for response — same as Go and TS.

---

## 5. Module and package design

- One package per directory; `__init__.py` exists but stays minimal — no logic, only re-exports if absolutely needed.
- Avoid `utils`, `common`, `helpers`, `misc` packages — split by domain.
- Internal symbols use a leading underscore. `__all__` only when you actively curate the public surface.
- No circular imports — extract the shared dependency.
- Avoid wildcard imports (`from x import *`).

---

## 6. Design patterns

- **Dependency injection over module-level singletons.** Inject clients, DB sessions, clocks, loggers into class `__init__` or function parameters. Module-level state is untestable without monkey-patching.
- **Accept protocols, return concrete types.** Define `Protocol` interfaces at the consumption point.
- **Composition over inheritance.** Multiple inheritance only for mixins with a single clear concern.
- **Avoid `__init__.py` side effects.** No I/O, no DB connections, no network at import time — same rule as Go's `init()`.
- **`@dataclass(frozen=True, slots=True)` by default** for value types — immutable, fast, no surprise.

See `~/.claude/agents/swe/principles.md` for the cross-language version of these rules.

---

## 7. Async patterns

- `asyncio` is the default concurrency model for I/O-bound code. Use `anyio` when supporting both `asyncio` and `trio` backends.
- Don't mix sync and async code paths in the same module. Pick a colour, stay there.
- Never `asyncio.run` inside a library — only at application entry points.
- `asyncio.gather(*tasks, return_exceptions=True)` for parallel I/O where partial failure is acceptable; otherwise plain `gather` and let it raise.
- Always cancel and await child tasks on shutdown. Orphaned tasks are bugs.
- `httpx.AsyncClient` over `aiohttp` unless the project already standardised on one.

---

## 8. Testing

- `pytest` only. No `unittest`.
- Test files colocate next to source: `order_service.py` → `test_order_service.py` in the same package.
- Use `pytest` fixtures for setup; avoid class-based test inheritance.
- Parametrize with `@pytest.mark.parametrize` instead of loops inside a test.
- Mock at module boundaries with `monkeypatch` or `unittest.mock` — never mock internals of the module under test.
- Test coverage target: >80% on business logic packages; 100% on security-critical paths.
- For DB-touching code, prefer integration tests against an ephemeral instance (`testcontainers`, `pytest-postgresql`) over heavy mocking.
- Async tests via `pytest-asyncio` with `asyncio_mode = "auto"`.

---

## 9. Concurrency

- CPU-bound work: `concurrent.futures.ProcessPoolExecutor`. Don't use threads for CPU work (GIL).
- I/O-bound work: prefer `asyncio` over threads.
- Threads only when interfacing with sync libraries from async code (`asyncio.to_thread`).
- Document task lifetimes — who starts it, what stops it, what happens on error.
- Use `contextvars` for per-task context (request ID, user); never module globals.

---

## 10. Performance

- Profile before optimising — `cProfile`, `pyinstrument`, `py-spy`.
- Avoid premature allocation: comprehensions over `append` loops; generators when streaming.
- Slots on hot dataclasses (`@dataclass(slots=True)`) — cuts memory and attribute access cost.
- Database queries: `EXPLAIN ANALYZE` before adding an index; N+1 queries are a bug. Same rule as Go and TS.
- Use `lru_cache` for pure functions with bounded input space. Document the bound.
