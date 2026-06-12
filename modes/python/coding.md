# Python Standards

Builds on `~/.claude/modes/go/coding.md` and `~/.claude/modes/ts/coding.md` — same bar, Python idioms. Idiomatic Python you already know is assumed; this covers only Komodo-specific or non-obvious points. Project configs may extend, not contradict.

---

## 1. Tooling & types (Komodo-specific)

- Python 3.12+ pinned in `pyproject.toml` (`requires-python = ">=3.12,<3.13"`). Env/deps via `uv` (preferred) or `poetry` — never raw `pip`.
- Format `ruff format`; lint `ruff check`; type-check `mypy --strict` (or strict `pyright`). No `black`/`isort`/`flake8`/`pylint` stack; type errors block merge.
- `from __future__ import annotations` at the top of every module. Annotate every signature and module-level binding. No `Any` without `# type: ignore[reason]`.
- `typing.Protocol` for boundary interfaces (ABCs only for genuine nominal subtyping). Validate external boundaries with `pydantic` v2 or `attrs`. Prefer `TypedDict`/`dataclass`/`BaseModel` over `dict[str, Any]`; `@dataclass(frozen=True, slots=True)` for value types.

---

## 2. Conventions

- Errors: catch specific exceptions (never bare `except:`); wrap with a descriptive phrase carrying no function name — `raise OrderError("failed to query order by ID") from err` (format: `principles.md` §1). Inherit one project base (`KomodoError`); log once at the propagation boundary. No `assert` for runtime checks (stripped under `-O`).
- Docstrings: default none; only with a `comments.md` license (why / public API / edge case), ≤2 sentences, never name-restating. Boolean prefixes `is_/has_/can_/should_`; `req`/`res` for HTTP request/response.
- Naming: `lower_snake_case` modules, `PascalCase` classes/aliases, `snake_case` functions/vars, `SCREAMING_SNAKE` constants, leading `_` for private.
- Minimal `__init__.py` (no logic, no import-time I/O); avoid `utils`/`common`/`helpers`; no circular or wildcard imports. DI over module-level singletons (`principles.md` §3).

---

## 3. Async

`asyncio` default for I/O-bound code (`anyio` for dual-backend); don't mix sync/async in one module. Never `asyncio.run` inside a library. `gather(..., return_exceptions=True)` when partial failure is acceptable; cancel and await child tasks on shutdown. `httpx.AsyncClient` over `aiohttp`. CPU-bound work → `ProcessPoolExecutor`, not threads (GIL). `contextvars` for per-task context.

---

## 4. Testing

`pytest` only (no `unittest`). Colocate `test_<name>.py` in the same package. Fixtures over class inheritance; `@pytest.mark.parametrize` over in-test loops. Mock at module boundaries (`monkeypatch`/`unittest.mock`), never internals. Async via `pytest-asyncio` (`asyncio_mode = "auto"`). DB-touching code → integration tests against ephemeral instances (`testcontainers`, `pytest-postgresql`). Coverage >80% business logic, 100% security-critical.
