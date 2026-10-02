package check

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/git"
)

// Snapshot is what an output check compares before and after a model session.
type Snapshot struct {
	Head   string            `json:"head"`
	Branch string            `json:"branch"`
	Refs   map[string]string `json:"refs"`
	Hooks  map[string]string `json:"hooks"`
	Config map[string]string `json:"config"`
}

// Ref prefixes other lanes write to concurrently: their own branches and komodo tips, and the remote-tracking refs.
const (
	branchRefs = "refs/heads/"
	remoteRefs = "refs/remotes/"
	tipRefs    = "refs/komodo/"
)

// branchSection prefixes the config keys git keeps per branch, as branch.<name>.<key>.
const branchSection = "branch."

// TakeSnapshot reads worktree's HEAD and branch, the refs no lane owns, and the shared git hooks and config.
func TakeSnapshot(worktree string) (Snapshot, error) {
	head, err := git.Run(worktree, "rev-parse", "HEAD")
	if err != nil {
		return Snapshot{}, err
	}
	branch := git.TrackedBranch(worktree)
	refs, err := snapshotRefs(worktree, branch)
	if err != nil {
		return Snapshot{}, err
	}
	gitDir, err := git.Run(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return Snapshot{}, err
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktree, gitDir)
	}
	hooks, err := snapshotHooks(filepath.Join(gitDir, "hooks"))
	if err != nil {
		return Snapshot{}, err
	}
	config, err := snapshotConfig(gitDir, branch)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Head: head, Branch: branch, Refs: refs, Hooks: hooks, Config: config}, nil
}

// snapshotRefs reads the commit hash of branch and of every ref outside other lanes' branches and the remotes.
func snapshotRefs(worktree, branch string) (map[string]string, error) {
	out, err := git.Run(worktree, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := fields[0]
		switch {
		case strings.HasPrefix(name, remoteRefs):
			continue
		case strings.HasPrefix(name, branchRefs) && name != branchRefs+branch:
			continue
		case strings.HasPrefix(name, tipRefs) && name != tipRefs+branch:
			continue
		}
		refs[name] = fields[1]
	}
	return refs, nil
}

// snapshotConfig reads every key of the config in gitDir, one value per line for a multi-valued key,
// skipping the branch sections of every branch but branch; a missing config reads as empty.
func snapshotConfig(gitDir, branch string) (map[string]string, error) {
	path := filepath.Join(gitDir, "config")
	config := map[string]string{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, nil
	}
	out, err := git.Run(gitDir, "config", "--file", path, "--null", "--list")
	if err != nil {
		return nil, err
	}
	for _, entry := range strings.Split(out, "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		if key == "" {
			continue
		}
		rest, inBranch := strings.CutPrefix(key, branchSection)
		if inBranch && strings.Contains(rest, ".") && !strings.HasPrefix(rest, branch+".") {
			continue
		}
		if prior, ok := config[key]; ok {
			value = prior + "\n" + value
		}
		config[key] = value
	}
	return config, nil
}

// snapshotHooks hashes every file in a hooks directory, keyed by its name; a missing directory hashes nothing.
func snapshotHooks(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	hooks := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		hash, err := hashFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		hooks[entry.Name()] = hash
	}
	return hooks, nil
}

// hashFile returns the sha256 of a file's contents, or an empty hash when the file does not exist.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Compare names every way after diverges from before: a moved HEAD, a switched branch, a changed or
// removed ref, a changed git hook, or a changed git config key.
func Compare(before, after Snapshot) []string {
	var problems []string
	if before.Head != after.Head {
		problems = append(problems, fmt.Sprintf("a model commit moved HEAD from %s to %s", before.Head, after.Head))
	}
	if before.Branch != after.Branch {
		problems = append(problems, fmt.Sprintf("HEAD switched from branch %q to %q", before.Branch, after.Branch))
	}
	problems = append(problems, compareRefs(before.Refs, after.Refs)...)
	problems = append(problems, compareHooks(before.Hooks, after.Hooks)...)
	for _, key := range unionKeys(before.Config, after.Config) {
		if before.Config[key] != after.Config[key] {
			problems = append(problems, fmt.Sprintf("the git config key %s changed", key))
		}
	}
	return problems
}

// compareRefs names every ref that moved, appeared, or disappeared between before and after.
func compareRefs(before, after map[string]string) []string {
	var problems []string
	for _, name := range unionKeys(before, after) {
		switch {
		case after[name] == "":
			problems = append(problems, fmt.Sprintf("ref %s was removed", name))
		case before[name] == "":
			problems = append(problems, fmt.Sprintf("ref %s was added", name))
		case before[name] != after[name]:
			problems = append(problems, fmt.Sprintf("ref %s changed", name))
		}
	}
	return problems
}

// compareHooks names every hook file that changed, was added, or was removed between before and after.
func compareHooks(before, after map[string]string) []string {
	var problems []string
	for _, name := range unionKeys(before, after) {
		if before[name] != after[name] {
			problems = append(problems, fmt.Sprintf("git hook %s changed", name))
		}
	}
	return problems
}

// unionKeys returns the sorted union of two maps' keys.
func unionKeys(a, b map[string]string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var keys []string
	for name := range a {
		seen[name] = true
		keys = append(keys, name)
	}
	for name := range b {
		if !seen[name] {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
}
