# Brief templates

One template per role in the roster. Each names the slots that role requires, the slots it accepts, and what belongs in each.

These are authoring references for work inside this repo. They are not installed — `scripts/install.py` links only the contents of `claude-code/` into `~/.claude/`, so a session in any other repo reads the delegate slot table in `workflow-loop/SKILL.md` instead. The two must agree; changing a required set here means changing it there.

## The contract every template shares

**A required slot that is absent and one that is present but empty are the same thing: the fork stops and returns the gap rather than guessing a value.** That return replaces the fork's whole output template — a brief that cannot be executed has no findings, no options, and no queue to report, so nothing else comes back with it.

**Delete an optional slot you have nothing for** instead of leaving it blank. An empty optional slot reads as an unanswered required one.

**Never write "as discussed".** A fork cannot see the calling conversation; whatever it needs goes in a slot or is on disk.

`scout.md.tmpl` states its own variant of the first rule, tied to `Task` alone — it is the one role where a ceremonial brief costs more than the search it describes.
