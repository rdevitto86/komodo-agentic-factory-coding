# Agent-name constants shared across hooks. REVIEWER_AGENT used to live in
# reviewer_guard.py; git_guard.py's Bash-side reviewer check still needs it
# now that reviewer_guard.py itself is retired.

REVIEWER_AGENT = "reviewer"

# These key git_guard.py's per-agent git subcommand policy, one identity path.
BUILDER_AGENT = "builder"
TESTER_AGENT = "tester"
SCOUT_AGENT = "scout"
RESEARCHER_AGENT = "researcher"
ARCHITECT_AGENT = "architect"
