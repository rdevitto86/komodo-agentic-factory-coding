---
name: standards-python
description: Python standards — tooling, typing, error handling, async, pytest. Load before reading or writing any .py, .pyi, or pyproject.toml file.
user-invocable: false
paths: "**/*.py, **/*.pyi, **/pyproject.toml"
---

# Python

Zero comments, zero docstrings. Errors lead with a verb phrase and never name the function.

## Comment discipline

`standards-comments` owns the rules and loads on every `.py` file alongside this skill — read it there, not here. In short: every function gets one line saying what it does, public or private; everything else gets nothing unless the code cannot say it; `comments.py apply` is the only write path. A docstring on the first body line already satisfies the rule — never add a `#` comment above a function that has one.

## Toolchain

- **The version floor is declared in `pyproject.toml` under `requires-python`.** Read it rather than assuming a release. Dependencies via `uv` (preferred) or `poetry` — never raw `pip`.
- **`ruff format`, `ruff check`, `mypy --strict`** (or strict `pyright`). No `black`/`isort`/`flake8`/`pylint` stack. Type errors block merge.
- **Vulnerability scanning** — `pip-audit` (or `uv pip audit`) against the resolved lockfile is the gate. Enable ruff's `S` (bandit) rules for the static half, excluded from test paths. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Outdated dependencies** — `uv pip list --outdated` (or `pip list --outdated`) lists packages behind the latest release, no CVE required to surface. Advisory only, never a merge gate; bump one package at a time and re-run the test suite, rather than a blanket upgrade.
- **Internal dependency graph** — the toolchain ships no package-level graph command. The closest thing that needs no install is the standard library's `python -m modulefinder <entry>`, which walks the imports reachable from one entry point; anything wider is a read of the `import`/`from` lines themselves. Do not reach for a third-party graph tool. `assess-change-risk` states when to run it and what its output does to the tier.
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
- **Depend on a `Protocol` (or an ABC) the caller defines, not a concrete class** — the Dependency Inversion Principle applied to Python. A consumer declares the narrow `Protocol` it needs and a concrete implementation is passed in; the implementation depends on the `Protocol`'s shape, never the reverse.
- **Wrap a long statement by collapsing one level at a time, never straight to one-arg-per-line.** Try the whole statement on one line first; if it doesn't fit, move the wrapped content to its own indented line(s) with a trailing comma and the closing paren/bracket/brace on its own line at the original indent — a function call may stay grouped at this step if it fits, but a dict/dataclass literal's fields never do, always one field per line once wrapped. Only if that grouped form is still too long, break to one item per line. Apply identically to calls, dict/list literals, and multi-arg logger calls.
- **A numeric or short repeated string literal standing for a size, TTL, count, threshold, or spec-level value (a header name, a status string) gets extracted to a named constant.** Module-level `SCREAMING_SNAKE` when shared by more than one function in the module; a function-local constant when scoped to one call. A literal repeated across 2+ files or functions is the strongest signal to extract first. Two or more constants introduced together go together in one grouping, not scattered standalone assignments.
- **No blank line between two consecutive early-return guard clauses.** Exactly one blank line between the last guard clause in a sequence and the happy-path logic that follows it.

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

This skill carries no `Repo layout — <token>` section. **Create is unsupported for Python** in `git-repo-init` — no repo type token maps here. Use `git-repo-init`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here first.
