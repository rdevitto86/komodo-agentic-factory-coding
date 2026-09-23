---
name: standards-python
description: Python: type hints, docstrings, exceptions, and the standard library first.
globs: ["**/*.py", "**/*.pyi"]
---

# Python

## Comments
- A docstring on every public function, class, and module: one line saying what it does or returns.
- A private function gets a docstring or one `#` line when its body is longer than a screen or its behaviour is not obvious.
- Error messages lead with a verb phrase and never name the function.

## Conventions
- Catch specific exceptions, never bare `except:`. Wrap with a phrase carrying no function name: `raise OrderError("failed to query order by id") from err`.
- One project base exception. Log once, at the propagation boundary.
- No `assert` for runtime checks.
- `lower_snake_case` modules, `PascalCase` classes, `snake_case` functions and variables, `SCREAMING_SNAKE` constants, leading `_` for private. Booleans take `is_`, `has_`, `can_`, `should_`.
- Minimal `__init__.py`: no logic, no import-time I/O. No `utils`, `common`, or `helpers` modules. No circular or wildcard imports.
- Inject dependencies. Depend on a `Protocol` or ABC the caller defines, not a concrete class.
- Wrap a long statement one level at a time: content on its own indented lines with a trailing comma, closer at the original indent.
- Extract a literal that stands for a size, TTL, count, threshold, header, or config key into a named constant. Module-level when shared, function-local otherwise. Constants introduced together stay together.
- No blank line between consecutive early-return guards; one blank line before the happy path.

## Async
- `asyncio` by default for I/O-bound code. Never mix sync and async in one module. Never call `asyncio.run` inside a library.
- `gather(..., return_exceptions=True)` when partial failure is acceptable. Cancel and await child tasks on shutdown.
- `httpx.AsyncClient` over `aiohttp`. CPU-bound work goes to `ProcessPoolExecutor`. `contextvars` for per-task context.

## Security
- `yaml.safe_load`, never `yaml.load`, on untrusted input.
- `pickle`, `marshal`, `shelve` never deserialize data from outside the process.
- `eval`/`exec` on request-derived text is code execution.
- `subprocess` with an argument list and `shell=False`, never an interpolated string with `shell=True`.
- Jinja2 autoescaping stays on; `|safe` and `Markup()` never wrap untrusted data.
- DB-API placeholders, never f-strings building query text.
- `secrets`, never `random`, for a token, key, or reset code.

## Testing
- `pytest` for application code. Unit tests colocate as `test_<name>.py` beside the module, with `importmode = "importlib"`.
- Every other tier lives under `tests/`, one optional subfolder per tier, flat by feature inside.
- Helpers: one file, bottom of that file; one directory, that directory's `conftest.py`; everywhere, the root `conftest.py`.
- Fixtures over class inheritance; `parametrize` over in-test loops; never `setup_method`. Avoid `autouse=True`.
- Scope fixtures by cost: pools, mock servers, and base config are `scope="session"` and read-only.
- Parallel via `pytest-xdist` for unit, component, contract; serial for smoke, integration, e2e, chaos.
- Poll, never `time.sleep`. Cheapen the test config, not the code. Roll back a transaction instead of truncating.
- Mock at module boundaries, never internals. Async via `pytest-asyncio` with `asyncio_mode = "auto"`.
- DB-touching code gets integration tests against ephemeral instances.
