# 0020. Extension points ship disabled: notifiers, tool packs and stage hooks

**Status:** Accepted, 2026-09-25.

**Context.** The owner plans Slack, Google Chat and cloud-command plugins later, and wants V1 ready for them.

**Decision.** V1 defines the three plugin types, each with a manifest, installs them disabled, and enables them per machine. A notifier copies blocker notes and run summaries somewhere else; it never decides anything.

**Alternatives.**

- **Build the integrations now.** None is needed for 1.0.0.

**Consequences.**

- **Adding Slack later is a plugin,** not a change to the conductor.
