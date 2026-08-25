#!/usr/bin/env bash
#
# release.sh - tag and push the version CHANGELOG.md declares as current.
#
# Run:     bash scripts/release.sh

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is dirty — commit all changes before releasing" >&2
  exit 1
fi

version=$(grep -m1 -E '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md | sed -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/')
if [[ -z "$version" ]]; then
  echo "error: CHANGELOG.md has no \"## [x.y.z]\" heading — add one before releasing" >&2
  exit 1
fi

tag="v$version"
if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "error: tag $tag already exists — bump the CHANGELOG.md heading to the new version first" >&2
  exit 1
fi

git tag "$tag"
git push origin "$tag"
echo "released $tag"
