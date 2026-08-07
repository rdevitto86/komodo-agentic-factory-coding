---
name: python
description: Python standards — tooling, typing, error handling, async, pytest. Load before reading or writing any .py, .pyi, or pyproject.toml file.
user-invocable: false
---

# Python

Zero comments, zero docstrings. Errors lead with a verb phrase and never name the function.

## Tooling and types

- **Python 3.12+**, pinned in `pyproject.toml` (`requires-python = ">=3.12,<3.13"`). Dependencies via `uv` (preferred) or `poetry` — never raw `pip`.
- **`ruff format`, `ruff check`, `mypy --strict`** (or strict `pyright`). No `black`/`isort`/`flake8`/`pylint` stack. Type errors block merge.
- **`from __future__ import annotations`** at the top of every module. Annotate every signature and module-level binding. No bare `Any`.
- **`typing.Protocol` for boundary interfaces**; ABCs only for genuine nominal subtyping.
- **Validate external boundaries** with `pydantic` v2 or `attrs`. Prefer `TypedDict` / `dataclass` / `BaseModel` over `dict[str, Any]`. Value types are `@dataclass(frozen=True, slots=True)`.

## Conventions

- **Catch specific exceptions**, never bare `except:`. Wrap with a descriptive phrase carrying no function name: `raise OrderError("failed to query order by id") from err`.
- **One project base exception.** Log once, at the propagation boundary.
- **No `assert` for runtime checks** — it is stripped under `-O`.
- **Naming**: `lower_snake_case` modules, `PascalCase` classes, `snake_case` functions and variables, `SCREAMING_SNAKE` constants, leading `_` for private. Booleans take `is_` / `has_` / `can_` / `should_`.
- **Minimal `__init__.py`** — no logic, no import-time I/O. Avoid `utils` / `common` / `helpers`. No circular or wildcard imports.
- **Inject dependencies** rather than reaching for module-level singletons.

## Async

- **`asyncio` by default** for I/O-bound code (`anyio` when both backends are needed). Do not mix sync and async in one module.
- **Never call `asyncio.run` inside a library.**
- **`gather(..., return_exceptions=True)`** when partial failure is acceptable. Cancel and await child tasks on shutdown.
- **`httpx.AsyncClient` over `aiohttp`.**
- **CPU-bound work goes to `ProcessPoolExecutor`**, not threads — the GIL makes threads useless here.
- **`contextvars`** for per-task context.

## Testing

- **`pytest` only**, never `unittest`.
- **Unit tests colocate** as `test_<name>.py` in the same package. Everything else moves to a top-level `test/integration/`, flat by feature (`test/integration/test_order.py`) — do not mirror the package tree.
- **Fixtures over class inheritance.** `@pytest.mark.parametrize` over in-test loops.
- **Mock at module boundaries** (`monkeypatch`, `unittest.mock`), never internals.
- **Async via `pytest-asyncio`** with `asyncio_mode = "auto"`.
- **DB-touching code gets integration tests** against ephemeral instances (`testcontainers`, `pytest-postgresql`).
- **Tier definitions, merge/release gates, and coverage floors are owned by the `sdlc` skill** (90% minimum on new code, 100% on SDKs/shared libraries and security-critical paths) — this section covers Python mechanics only.
