"""Test package; strips the git variables a hook exports so a suite run from a hook cannot reach the real repo."""

import os

# git exports these to every hook, and a `git` call inheriting them acts on the hook's repo, not the fixture's.
INHERITED_GIT_VARS = (
    "GIT_DIR",
    "GIT_WORK_TREE",
    "GIT_COMMON_DIR",
    "GIT_INDEX_FILE",
    "GIT_OBJECT_DIRECTORY",
    "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    "GIT_QUARANTINE_PATH",
    "GIT_PREFIX",
    "GIT_NAMESPACE",
)

for _name in INHERITED_GIT_VARS:
    os.environ.pop(_name, None)
