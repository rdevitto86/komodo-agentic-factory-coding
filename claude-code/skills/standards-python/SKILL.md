---
name: standards-python
description: Python standards — tooling, typing, error handling, async, pytest. Load before reading or writing any .py, .pyi, or pyproject.toml file.
user-invocable: false
paths: "**/*.py, **/*.pyi, **/pyproject.toml"
---

# Python

Zero comments, zero docstrings. Errors lead with a verb phrase and never name the function.

## Comment discipline

You must strictly limit code comments. **A non-compliant comment prompts the user for approval before the write lands — it does not fail outright.** That is deliberate while these directives are still being tuned: write only a comment you actually believe is warranted, since every miss costs the user a decision. **Deleting a comment you did not add always prompts too**, regardless of shape — moving or refactoring code is not licence to drop someone else's note.

Banned: a name echo (the comment's first word repeats the function/variable/class name below it); an implementation narrative (explaining *what* code is doing, or describing standard syntax); a redundant docstring for an internal/private utility not explicitly requested.

Allowed only: a compiler/linter directive (always allowed); a step marker (indented, inside a function body, <= 80 chars); a banner/section break (<= 40-char label); an intent/WHY comment using the `WHY:`, `NOTE:`, or `TODO(author/issue):` prefix.

This language's exempt machine directives, verified against the guard's own list: `# noqa`, `# type: ignore`, `# pylint:`, `# mypy:`, `# pyright:`, `# ruff:`, `# isort:`, `-*- coding` on line 1. **A docstring is scanned like any other comment** — the guard parses the file's AST, so it catches module, function, and class docstrings, not just `#` lines. Anything outside those prompts for approval.

## Toolchain

- **The version floor is declared in `pyproject.toml` under `requires-python`.** Read it rather than assuming a release. Dependencies via `uv` (preferred) or `poetry` — never raw `pip`.
- **`ruff format`, `ruff check`, `mypy --strict`** (or strict `pyright`). No `black`/`isort`/`flake8`/`pylint` stack. Type errors block merge.
- **Vulnerability scanning** — `pip-audit` (or `uv pip audit`) against the resolved lockfile is the gate. Enable ruff's `S` (bandit) rules for the static half, excluded from test paths. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Outdated dependencies** — `uv pip list --outdated` (or `pip list --outdated`) lists packages behind the latest release, no CVE required to surface. Advisory only, never a merge gate; bump one package at a time and re-run the test suite, rather than a blanket upgrade.
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

## Security standards

Language-specific insecure-usage patterns for `/assess-security` to pull from, beyond `standards-api-security`'s generic OWASP checklist.

- **`yaml.safe_load`, never `yaml.load`, on untrusted input.** The default `Loader` can instantiate arbitrary Python objects — an insecure-deserialization sink.
- **`pickle`/`marshal`/`shelve` never deserialize data from outside the process.** Unpickling is arbitrary code execution by design; there is no safe-mode flag to reach for instead.
- **`eval`/`exec` on any request-derived string is code execution**, not a shortcut — no amount of upstream validation makes it safe.
- **`subprocess` with `shell=True` and an interpolated string is command injection.** Pass an argument list with `shell=False` (the default) instead.
- **Jinja2 autoescaping stays on; the `|safe` filter and `Markup()` never wrap untrusted data** — either bypasses the auto-escaping that prevents template-driven XSS/SSTI.
- **DB-API parameter placeholders (`%s`, `?`), never an f-string or `.format()` building the query text** — string-built SQL is injectable independent of the driver.
- **`secrets`, never `random`, for a token, key, or password-reset code.** `random` is a Mersenne Twister — predictable once enough output is observed.

## Testing

- **`pytest` only**, never `unittest`.
- **Unit tests colocate** as `test_<name>.py` beside the module they cover. This requires `importmode = "importlib"` under `[tool.pytest.ini_options]` — with the default `prepend` mode, two same-named test modules in different packages collide.
- **Every other tier lives under a top-level `tests/`** — plural, the root pytest and the packaging tools expect. One optional subfolder per tier (`component`, `contract`, `integration`, `e2e`, `smoke`, `perf`, `chaos`), created only when that tier has tests. Flat by feature inside each; do not mirror the package tree.
- **Helper placement maps to pytest's own scoping**: one file → bottom of that file, private, under `# --- Helpers ---`; one directory → that directory's `conftest.py`; everywhere → the root `conftest.py` or a `tests/helpers.py`.
- **Descriptions are `#` comments, never docstrings.** A docstring in a test is still a banned comment; a one-or-two-line note above the `def` is the permitted form, and it is optional.
- **Parallel is preferred for unit, component, and contract — never mandatory** — via `pytest-xdist` (`-n auto`). A case that cannot share a worker carries `@pytest.mark.serial` and runs in its own pass; a suite left fully serial is fine.
- **Serial for smoke, integration, e2e, and chaos** — drop `-n` entirely for those tiers, since they call deployed infrastructure that the fan-out would load.
- **Fixtures over class inheritance.** `@pytest.mark.parametrize` over in-test loops. Never `setup_method` / `teardown_method` — a requested fixture is already lazy and explicit, which the xUnit hooks are not.
- **`autouse=True` is the thing to avoid**, not fixtures generally. It reintroduces the implicit per-case hook every other language is trying to escape, firing for tests that never asked for it.
- **Scope by cost, not by habit.** Connection pools, mock servers, and base config are `scope="session"` and treated as read-only; only genuinely mutated state stays `scope="function"`. A case needing a mutated copy does `dataclasses.replace` or `copy.deepcopy` locally.
- **`pytest-xdist` runs processes, not threads** — which is exactly why `monkeypatch.setenv` is safe under it and why a threaded runner would not be. That isolation costs per-worker startup; it is worth it.
- **Poll, never `time.sleep`.** A tight loop against a hard deadline (or `tenacity` with a short wait) returns as soon as the condition holds; a fixed delay is both slower and flaky under CI load.
- **Cheapen the test config, not the code** — bcrypt/Argon2 at the minimum work factor, `httpx` timeouts at 50–100ms, and logging routed to a null handler, all injected through the constructor rather than branched on an env check inside the module under test.
- **Roll back instead of truncating.** Bind the session to an outer `connection.begin()` and roll it back in fixture teardown; per-case cleanup goes from hundreds of milliseconds to near zero and cannot leave orphan rows behind a crashed test.
- **Mock at module boundaries** (`monkeypatch`, `unittest.mock`), never internals.
- **Async via `pytest-asyncio`** with `asyncio_mode = "auto"`.
- **DB-touching code gets integration tests** against ephemeral instances (`testcontainers`, `pytest-postgresql`).
- **Tier definitions, merge/release gates, and coverage floors are owned by the `standards-sdlc` skill** — this section covers Python mechanics only.

## Quick-reference fields

The field set a Python repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Version floor | `requires-python` |
| Package name | `pyproject.toml` |
| Dependency manager | lockfile present |
| Entrypoint | `project.scripts` |
| Run + test | `pyproject.toml` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for Python** in `repo-init` — no repo type token maps here. Use `repo-init`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here first.
