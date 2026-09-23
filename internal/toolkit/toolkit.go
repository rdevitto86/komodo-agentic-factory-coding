// Package toolkit serves the komodo/ tree every loader reads: roles, schemas, standards, skills,
// rules, policy, and facets, from disk when a repo ships its own, from the binary otherwise.
package toolkit

import (
	"io/fs"
	"os"
	"path/filepath"

	"komodo"
)

// embedded is the komodo/ tree shipped inside the binary, rooted where a repo's own komodo/ is.
var embedded = mustSub(komodo.FS, "komodo")

// markers are the toolkit's top-level entries; a komodo/ directory holding none is another package.
var markers = []string{"roles", "skills", "rules", "facets", "policy.json", "AGENTS.md"}

// FS returns the komodo/ tree for root: its own komodo/ directory when that holds one of the
// toolkit's markers, otherwise the tree embedded in the binary, so a stray directory never blanks a role.
func FS(root string) fs.FS {
	dir := filepath.Join(root, "komodo")
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return os.DirFS(dir)
		}
	}
	return embedded
}

// mustSub roots an embedded tree at one directory the embed directive guarantees exists.
func mustSub(tree fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(tree, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
