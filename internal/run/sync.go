package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/doctor"
	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/line"
)

// BuiltFrom is the file beside the built binary that records the commit it was built from.
const BuiltFrom = ".built-from"

// buildLocal and installHooks build the binary and write the hooks; a test swaps them to skip the compile.
var (
	buildLocal   = gate.BuildLocal
	installHooks = gate.Install
)

// SyncOptions are what one sync needs: the root, whether to write, and where to print.
type SyncOptions struct {
	Root   string
	DryRun bool
	Stdout io.Writer
}

// Sync brings the root up to origin, rebuilds a stale toolkit binary, and re-renders drifted config,
// printing one line per step; it returns the rebuilt binary's path, or "" when it rebuilt nothing.
func Sync(options SyncOptions) (string, error) {
	out := options.Stdout
	if out == nil {
		out = os.Stdout
	}
	suffix := ""
	if options.DryRun {
		suffix = " (dry run)"
	}
	head, err := syncRoot(options.Root, options.DryRun, out, suffix)
	if err != nil {
		return "", err
	}
	built, err := syncBinary(options.Root, head, options.DryRun, out, suffix)
	if err != nil {
		return "", err
	}
	if err := syncConfig(options.Root, options.DryRun, out, suffix); err != nil {
		return "", err
	}
	return built, nil
}

// syncRoot fetches origin and fast-forwards a clean default branch to it, returning the commit HEAD ends on;
// a dry run fetches nothing and returns the commit HEAD would end on.
func syncRoot(root string, dryRun bool, out io.Writer, suffix string) (string, error) {
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if !dryRun {
		if _, err := git.Run(root, "fetch", "origin"); err != nil {
			fmt.Fprintf(out, "root: skipped, cannot fetch origin: %v\n", err)
			return head, nil
		}
	}
	base := line.DefaultBase(root)
	branch := git.Or(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branch != base {
		fmt.Fprintf(out, "root: skipped, on %q, not the default branch %q%s\n", branch, base, suffix)
		return head, nil
	}
	dirty, err := git.Run(root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", err
	}
	if dirty != "" {
		fmt.Fprintf(out, "root: skipped, the working tree has uncommitted changes%s\n", suffix)
		return head, nil
	}
	upstream, err := git.Run(root, "rev-parse", "--verify", "refs/remotes/origin/"+base)
	if err != nil {
		fmt.Fprintf(out, "root: skipped, origin has no branch %q%s\n", base, suffix)
		return head, nil
	}
	common := git.Or(root, "merge-base", head, upstream)
	if common == upstream {
		fmt.Fprintf(out, "root: already current%s\n", suffix)
		return head, nil
	}
	if common != head {
		fmt.Fprintf(out, "root: skipped, %s has commits origin lacks%s\n", base, suffix)
		return head, nil
	}
	if !dryRun {
		if _, err := git.Run(root, "merge", "--ff-only", upstream); err != nil {
			return "", err
		}
	}
	fmt.Fprintf(out, "root: updated %s..%s%s\n", short(head), short(upstream), suffix)
	return upstream, nil
}

// syncBinary rebuilds the toolkit's own built binary and rewrites the hooks when the binary's
// recorded source commit is not head, returning the rebuilt binary's path, or "" when it did not rebuild.
func syncBinary(root, head string, dryRun bool, out io.Writer, suffix string) (string, error) {
	if _, err := os.Stat(filepath.Join(root, "cmd", "komodo", "main.go")); err != nil {
		fmt.Fprintf(out, "binary: skipped, not the toolkit's own checkout%s\n", suffix)
		return "", nil
	}
	target := gate.LocalTarget()
	binPath := filepath.Join(root, "bin", target.Name)
	if _, err := os.Stat(binPath); err != nil {
		fmt.Fprintf(out, "binary: skipped, no %s is built; komodo gate --install builds it%s\n", target.Name, suffix)
		return "", nil
	}
	marker := filepath.Join(root, "bin", BuiltFrom)
	if built, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(built)) == head {
		fmt.Fprintf(out, "binary: already current%s\n", suffix)
		return "", nil
	}
	if dryRun {
		fmt.Fprintf(out, "binary: rebuilt %s from %s%s\n", target.Name, short(head), suffix)
		return "", nil
	}
	built, err := buildLocal(root, io.Discard)
	if err != nil {
		return "", err
	}
	dirty, err := git.Run(root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", err
	}
	if dirty != "" {
		fmt.Fprintf(out, "binary: skipped stamp, the working tree has uncommitted changes%s\n", suffix)
		return "", nil
	}
	if err := os.WriteFile(marker, []byte(head+"\n"), 0o644); err != nil {
		return "", err
	}
	common, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if _, err := installHooks(common); err != nil {
		return "", err
	}
	fmt.Fprintf(out, "binary: rebuilt %s from %s%s\n", target.Name, short(head), suffix)
	return built, nil
}

// syncConfig re-renders every installed host's whole config at the root when the doctor reports drift.
func syncConfig(root string, dryRun bool, out io.Writer, suffix string) error {
	problems, err := doctor.Run(root, doctor.Options{NoGit: true})
	if err != nil {
		return err
	}
	drifted := false
	for _, problem := range problems {
		drifted = drifted || problem.Check == "drift"
	}
	if !drifted {
		fmt.Fprintf(out, "config: already current%s\n", suffix)
		return nil
	}
	if !dryRun {
		if err := line.RenderRoot(root); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "config: re-rendered%s\n", suffix)
	return nil
}

// short is the first twelve characters of a commit hash.
func short(commit string) string {
	const width = 12
	if len(commit) > width {
		return commit[:width]
	}
	return commit
}
