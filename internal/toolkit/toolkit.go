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

// FS returns the komodo/ tree for root: its own komodo/ directory when the repo ships one,
// otherwise the tree embedded in the binary.
func FS(root string) fs.FS {
	dir := filepath.Join(root, "komodo")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return os.DirFS(dir)
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
