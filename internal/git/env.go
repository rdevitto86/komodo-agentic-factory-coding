package git

import (
	"slices"
	"strings"
)

// RepoPointers are the variables git sets for a hook; inherited, they aim a git call away from its own directory.
var RepoPointers = []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_PREFIX", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR"}

// WithoutRepoPointers is env without RepoPointers, so git works on the repository its directory names.
func WithoutRepoPointers(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(RepoPointers, key) {
			out = append(out, entry)
		}
	}
	return out
}
