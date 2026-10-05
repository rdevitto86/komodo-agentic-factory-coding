// Package check reruns everything a build or repair session must satisfy before a review starts.
package check

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/proc"
)

// CommandTimeout bounds how long one group check may run.
const CommandTimeout = proc.DefaultTimeout

// Group is the worktree, base ref, and declared file scope one check run covers.
type Group struct {
	Worktree string
	Base     string
	Files    []string
}

// Run reruns the group's own checks and scope, and returns one problem per failure.
// Scope is checked last, since a passing build with a scope violation still needs every problem named.
func Run(g Group, checks []string) []string {
	return RunContext(context.Background(), g, checks)
}

// RunContext is Run whose commands are killed once ctx is done.
func RunContext(ctx context.Context, g Group, checks []string) []string {
	var problems []string
	for _, command := range checks {
		problems = append(problems, runNamed(ctx, g.Worktree, "check", command)...)
	}
	problems = append(problems, Scope(g.Worktree, g.Base, g.Files)...)
	return problems
}

// runNamed runs one command under name and reports its failure, skipping an empty command.
func runNamed(ctx context.Context, worktree, name, command string) []string {
	if command == "" {
		return nil
	}
	argv := proc.ShellArgv(command)
	ran := proc.ExecContext(ctx, worktree, CommandTimeout, argv[0], argv[1:]...)
	if ran.OK() {
		return nil
	}
	return []string{fmt.Sprintf("%s: `%s` %v\n%s", name, command, ran.Err(), ran.Output)}
}

// Scope names every file the working tree changes against base, untracked ones included,
// outside the group's declared files.
func Scope(worktree, base string, files []string) []string {
	if base == "" {
		return nil
	}
	changed, err := changedFiles(worktree, base)
	if err != nil {
		return []string{"scope: " + err.Error()}
	}
	allowed := make(map[string]bool, len(files))
	for _, file := range files {
		allowed[file] = true
	}
	var problems []string
	for _, name := range changed {
		// A declared file's own test is in scope too, as the builder's rules allow.
		if allowed[name] || allowed[testedBy(name)] || allowed[untagged(testedBy(name))] ||
			underDeclared(name, files) || testsDeclaredPackage(name, files) || onlyTicks(worktree, base, name) {
			continue
		}
		problems = append(problems, fmt.Sprintf("scope: %s is edited outside the group's declared files", name))
	}
	return problems
}

// underDeclared reports whether name sits inside a declared directory, such as a skill folder a task removes.
func underDeclared(name string, files []string) bool {
	for _, file := range files {
		if dir := strings.TrimSuffix(file, "/"); dir != "" && strings.HasPrefix(name, dir+"/") {
			return true
		}
	}
	return false
}

// testSuffixes map a test file's ending to the ending of the source file it tests.
var testSuffixes = [][2]string{
	{"_test.go", ".go"}, {".test.ts", ".ts"}, {".spec.ts", ".ts"}, {".test.tsx", ".tsx"}, {".spec.tsx", ".tsx"},
}

// testedBy is the source file a test file tests, or empty when name is not a test file.
func testedBy(name string) string {
	for _, pair := range testSuffixes {
		if strings.HasSuffix(name, pair[0]) {
			return strings.TrimSuffix(name, pair[0]) + pair[1]
		}
	}
	return ""
}

// testsDeclaredPackage reports whether name is a Go test in a directory holding a declared Go file,
// since a package's change can move any of that package's tests.
func testsDeclaredPackage(name string, files []string) bool {
	if !strings.HasSuffix(name, "_test.go") {
		return false
	}
	dir := path.Dir(name)
	for _, file := range files {
		if strings.HasSuffix(file, ".go") && path.Dir(file) == dir {
			return true
		}
	}
	return false
}

// platformTags are the Go file-name suffixes that restrict a file to one system or architecture.
var platformTags = map[string]bool{
	"unix": true, "windows": true, "darwin": true, "linux": true, "freebsd": true, "openbsd": true,
	"netbsd": true, "dragonfly": true, "solaris": true, "illumos": true, "aix": true, "plan9": true,
	"android": true, "ios": true, "js": true, "wasip1": true, "amd64": true, "arm64": true, "386": true,
	"arm": true, "wasm": true, "riscv64": true, "ppc64le": true, "s390x": true, "mips64": true,
}

// untagged strips a Go source name's platform suffixes, so evidence_unix.go names evidence.go.
func untagged(name string) string {
	if !strings.HasSuffix(name, ".go") {
		return name
	}
	stem := strings.TrimSuffix(name, ".go")
	for range 2 {
		cut := strings.LastIndex(stem, "_")
		if cut <= strings.LastIndex(stem, "/")+1 || !platformTags[stem[cut+1:]] {
			break
		}
		stem = stem[:cut]
	}
	return stem + ".go"
}

// diffArgs is the git diff invocation, prefix, colour, and quoting pinned so parsing never
// depends on the caller's git config.
func diffArgs(args ...string) []string {
	return append([]string{
		"-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/",
	}, args...)
}

// changedFiles lists every file the working tree changes since it forked from base, then every untracked file.
func changedFiles(worktree, base string) ([]string, error) {
	fork, err := mergeBase(worktree, base)
	if err != nil {
		return nil, err
	}
	tracked, err := git.Run(worktree, diffArgs("--name-only", fork)...)
	if err != nil {
		return nil, err
	}
	untracked, err := untrackedFiles(worktree)
	if err != nil {
		return nil, err
	}
	return append(splitLines(tracked), untracked...), nil
}

// onlyTicks reports whether name is a backlog group file whose every change since the fork flips a task's checkbox.
func onlyTicks(worktree, base, name string) bool {
	if path.Dir(name) != backlog.GroupFilesDir || path.Ext(name) != ".md" {
		return false
	}
	fork, err := mergeBase(worktree, base)
	if err != nil {
		return false
	}
	out, err := git.Run(worktree, diffArgs("--unified=0", fork, "--", name)...)
	if err != nil || out == "" {
		return false
	}
	var removed, added []string
	inHunk := false
	for _, text := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(text, "@@"):
			inHunk = true
		case !inHunk:
		case strings.HasPrefix(text, "-"):
			removed = append(removed, strings.Replace(text[1:], "- [ ]", "- [x]", 1))
		case strings.HasPrefix(text, "+"):
			added = append(added, text[1:])
		}
	}
	return len(removed) > 0 && slices.Equal(removed, added)
}

// mergeBase is the newest commit HEAD forked from base, its origin copy, or the line's tip of it,
// so commits landing on base since never count as edits.
func mergeBase(worktree, base string) (string, error) {
	var fork string
	var firstErr error
	for _, ref := range []string{base, "refs/remotes/origin/" + base, "refs/komodo/" + base} {
		candidate, err := git.Run(worktree, "merge-base", ref, "HEAD")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if fork == "" {
			fork = candidate
		} else if _, err := git.Run(worktree, "merge-base", "--is-ancestor", fork, candidate); err == nil {
			fork = candidate
		}
	}
	if fork == "" {
		return "", firstErr
	}
	return fork, nil
}

// untrackedFiles lists the files git does not track and does not ignore.
func untrackedFiles(worktree string) ([]string, error) {
	out, err := git.Run(worktree, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// splitLines returns the non-blank, trimmed lines of out.
func splitLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// Diff is the working tree's unified diff since it forked from base, each untracked file rendered as wholly added.
func Diff(worktree, base string) (string, error) {
	fork, err := mergeBase(worktree, base)
	if err != nil {
		return "", err
	}
	tracked, err := git.Run(worktree, diffArgs(fork)...)
	if err != nil {
		return "", err
	}
	untracked, err := untrackedFiles(worktree)
	if err != nil {
		return "", err
	}
	var diff strings.Builder
	diff.WriteString(tracked)
	diff.WriteString("\n")
	for _, name := range untracked {
		data, err := os.ReadFile(filepath.Join(worktree, name))
		if err != nil {
			return "", err
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		fmt.Fprintf(&diff, "--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", name, len(lines))
		for _, line := range lines {
			diff.WriteString("+" + line + "\n")
		}
	}
	return diff.String(), nil
}
