// Package komodo embeds the komodo/ tree the binary ships when a target repo carries none of its own.
package komodo

import "embed"

// FS is the komodo/ tree: roles, schemas, skills, rules, policy, and facets, rooted at "komodo".
//
//go:embed komodo
var FS embed.FS

// Templates is the starter tree komodo init writes into a new repo, rooted at "templates/project".
//
//go:embed all:templates/project
var Templates embed.FS
