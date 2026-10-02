# 0036. A pull request opens through `komodo pr create`, which checks its title and applies its labels

**Status:** Accepted, 2026-10-02.

**Context.** Only the line's ship code labelled a pull request. A session or subagent that ran `gh pr create` itself left it bare: across four repos on 2026-10-02, 17 of the last 63 pull requests carried no label and 6 more lacked `@agent`, nearly all of them opened by hand. No rule, skill or template asked for labels, and nothing checked a title. The scope rules were also this repo's own paths, so a product repo always earned `scope/harness`, a label it does not define.

**Decision.** `komodo pr create` opens a pull request from the current branch, refuses a title that is not `<type>: <summary>` within 72 characters, and applies `@agent`, the scope the branch's files earn, the stage of the repo's newest changelog version, and `branch/feature` when the base is not the default branch. `komodo pr label` does the same for a pull request opened another way. A repo maps its own scopes in `.komodo/labels.json`; without one it gets the toolkit's rules. The guard refuses a model's `gh pr create` and names the command to use. Ship applies the same stage and branch labels, and neither warns when a repo lacks one.

**Alternatives.**

- **A rule in `AGENTS.md` alone.** The rule-free version already failed most of the time; a rule a model can skip is the same gap.
- **A pre-push gate check for an unlabelled pull request.** The gate is local and never calls the forge (0018).
- **Labels from a GitHub Action.** Nothing runs on the forge (0018).

**Consequences.**

- **Every pull request a session opens is labelled and titled to the template,** or the guard says why not.
- **A person can still run `gh pr create`;** the guard judges only a model's tool calls.
- **A product repo without `.komodo/labels.json` earns the toolkit's scope labels,** which it may not define; ship and `komodo pr` warn on the miss.
