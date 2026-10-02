# 0025. Sessions isolate by flags, keep the default config directory, and cap turns

**Status:** Accepted, 2026-09-26. Amends 0006.

**Context.** Decisions 0005 and 0006 hold only if spikes S2, S3, S4, S5, S7 and S8 pass. All six ran on 2026-09-26 on macOS with CLI 2.1.283 on a Pro subscription. Four held as written; S2, S3 and S8 each changed one part of the design.

- **Spike S2 result:** passes once the conductor sets `GOCACHE` and `GOTMPDIR` inside the worktree. A Sonnet builder under `dontAsk`, an allow list and the sandbox then built and tested a Go module in 5 turns with 0 denials. Without them the sandbox refused Go's cache in `~/Library/Caches`, and `dontAsk` denied all 6 of the builder's workarounds, disabling the sandbox among them. With `autoAllowBashIfSandboxed`, sandboxed commands outside the allow list, such as `cat` and `ls`, ran: the sandbox is the wall, not the Bash allow list.
- **Spike S3 result:** fails as designed and passes by flags. A fresh `CLAUDE_CONFIG_DIR` logs the host out (`claude auth status`: `loggedIn: false`), since a subscription login is keyed to the default directory. The default directory with `--setting-sources project,local` and `--strict-mcp-config` kept the login and shut out the personal `CLAUDE.md` (a canary phrase was absent), 3 personal plugins, 7 personal agents and 5 MCP servers. Left in: 2 built-in plugins, 15 built-in skills, 5 built-in agents and an auto-memory directory. `--disable-slash-commands` also drops the role plugin's skills, and `--setting-sources local` also drops the worktree's `CLAUDE.md`.
- **Spike S4 result:** passes. The result event carries `num_turns`, `usage`, `total_cost_usd`, `modelUsage` and `session_id`, and `rate_limit_event` carries `five_hour` and `seven_day` utilisation with `resetsAt`. `--resume <session_id>` recalled a number from the first session and kept its ID; `num_turns` counts only the resumed invocation. TSK-05.2.3 records its own fixtures the same way, with local paths and account fields removed.
- **Spike S5 result:** passes. A 38-turn Sonnet builder session with the builder's `--json-schema` wrote 12 files across 6 packages and returned a `structured_output` object matching the schema: `DONE`, 12 changed files, 19 verified commands, all exit 0. An independent `go test ./...` agreed.
- **Spike S7 result:** passes. Phase 0's drain was started from an interactive session through its shell tool; it launched 4 headless group sessions of its own and opened pull requests #208 to #211.
- **Spike S8 result:** `CLAUDE_CODE_MAX_TURNS=2` ends a session after 2 tool calls with subtype `error_max_turns`. `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1` removed `AWS_SECRET_ACCESS_KEY` from the Bash tool's environment but left `GH_TOKEN` and `GITHUB_TOKEN`, and it forced `dontAsk` back to the default mode with a warning.

**Decision.**

- **A line session keeps the host's default config directory** and isolates by flags: `--setting-sources project,local`, `--strict-mcp-config`, `--plugin-dir` for the role and `--settings` for its permissions and auto-memory. `DISABLE_AUTOUPDATER=1` moves from the rendered directory into the session's environment.
- **The conductor sets Go's caches inside the worktree,** gitignored, so a sandboxed build writes only there. Spike S1 still owns module downloads and race tests.
- **`CLAUDE_CODE_MAX_TURNS` caps each role's turns; `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB` is never set.** The conductor keeps removing forge credentials itself, as `run.Scrub` does today.

**Alternatives.**

- **A line-owned directory with `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`.** The line would store a long-lived login secret, and the built-ins would still load.
- **The host's bare mode.** It never reads a subscription login.

**Consequences.**

- **15 built-in skills stay in every session's context.** Doctor reads a session's init event to prove nothing personal loaded.
- **TSK-05.2.2 and TSK-05.3.2 change:** no `CLAUDE_CONFIG_DIR`; the flags above, the Go caches, a turn cap and auto-memory off instead.
