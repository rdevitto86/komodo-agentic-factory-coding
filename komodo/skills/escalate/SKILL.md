---
name: escalate
description: Settle one escalated group with exactly one allowed action, inside its limits; anything else is a stop.
---

# Escalate

A group stopped: its builder is blocked, it reached its time limit, a loop stopped making progress, or a stage failed in a way it can't retry. You settle that one escalation with exactly one action, and return it as the JSON the orchestrator role's schema names.

## The order

1. **Read why it stopped.** The escalation names the group, the state it left and the reason: the builder's question, or the failure.
2. **Read the group's task list, the specs it cites, and the code its tasks name.** Nothing else is a source.
3. **Pick the one action whose limits hold.** When none does, the action is `stop`.
4. **Return the JSON.** `action` and a one-sentence `why`, plus `answer` or `needs` as the action asks.

## The actions and their limits

| Action | Do | Limits |
|---|---|---|
| `answer` | Answer the builder's question in `answer`; the resumed builder reads it | Only from the task list, the specs and the code |
| `split` or `clarify` | Rewrite the group's tasks in its backlog file on the group's branch; say what changed in `answer` | The rewrite must pass `komodo lint`; the conductor lints it, and a failure is a stop |
| `retry` | The builder runs again on the heavy tier | Once per group; a second retry is a stop |
| `stop` | The group waits for a person; name the one decision they must make in `needs` | The conductor writes the blocker note and publishes a blocked draft PR |

## Rules

- **An answer comes only from the task list, the specs and the code.** A guess, a preference or an outside fact is not an answer.
- **Anything that changes scope is a stop.** A new file, a new dependency, a changed interface or a task the list does not hold waits for a person.
- **One action, once.** Never return two, and never retry an action the conductor refused.
- **A group that stops twice without progress is blocked,** whatever you return.
- **Never run a git command that changes state,** and never edit a file other than the group's backlog file.
- **`why` and `needs` are for a person** reading the blocker note: one sentence each, naming the task and the decision.
