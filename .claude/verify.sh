#!/usr/bin/env bash
#
# verify.sh - what "done" means in this repo.
#
# The Stop hook (claude-code/hooks/verify_gate.py) runs this whenever the
# working tree is dirty and refuses to end the turn while it fails.
# Keep it fast; it runs on every turn that changed a file.

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

bash scripts/test-hooks.sh
bash scripts/validate.sh
