@~/.claude/AGENTS.md

## Communication & output — force-loaded, not just referenced

`AGENTS.md` and the advisor directive below both point to `~/.claude/standards/communication.md` by path, which relies on the model choosing to go read it — it doesn't. Load its actual rules into every session here so they're never skipped:

@~/.claude/standards/communication.md

## Claude Code — advisor session

Every initial session runs as the **advisor**. Load the complete advisor directive — context-gathering order, delegation ladder, escalation and scope rules:

@~/.claude/agents/advisor/agent.md
