package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/doctor"
	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/line"
	"komodo/internal/profile"
)

// BuiltFrom is the file beside the built binary that records the commit it was built from.
const BuiltFrom = ".built-from"

const (
	// releaseURL is where published releases download from; KOMODO_RELEASE_URL overrides it, as install does.
	releaseURL = "https://github.com/rdevitto86/komodo-agentic-factory-coding/releases/download"
	// releaseSums is the checksum manifest every published release carries.
	releaseSums = "SHA256SUMS"
	// fetchTimeout bounds one release download.
	fetchTimeout = 5 * time.Minute
	// fetchLimit bounds one downloaded asset, in bytes.
	fetchLimit = 256 << 20
)

// buildLocal, installHooks and fetch build, write the hooks and download; a test swaps them out.
var (
	buildLocal   = gate.BuildLocal
	installHooks = gate.Install
	fetch        = httpGet
)

// SyncOptions are what one sync needs: the root, whether to write, and where to print.
type SyncOptions struct {
	Root   string
	DryRun bool
	Stdout io.Writer
}

// Sync brings the root up to origin, rebuilds a stale toolkit binary or fetches the pinned release elsewhere,
// and re-renders drifted config, printing one line per step; it returns the new binary's path, or "".
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
		return syncRelease(root, dryRun, out, suffix)
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
	common, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	built, stamped, err := gate.Stamp(root, common, head, buildLocal, installHooks, io.Discard)
	if err != nil {
		return "", err
	}
	if !stamped {
		fmt.Fprintf(out, "binary: skipped stamp, the working tree has uncommitted changes%s\n", suffix)
		return "", nil
	}
	fmt.Fprintf(out, "binary: rebuilt %s from %s%s\n", target.Name, short(head), suffix)
	return built, nil
}

// syncRelease installs the profile's pinned release into ~/.komodo/bin when another runs there, once its
// checksum matches the release's SHA256SUMS; it returns the installed path, or "" when it installed nothing.
func syncRelease(root string, dryRun bool, out io.Writer, suffix string) (string, error) {
	pin := doctor.PinnedRelease(root, profile.Select(root).Mode)
	if pin == "" {
		fmt.Fprintf(out, "binary: skipped, the profile pins no release%s\n", suffix)
		return "", nil
	}
	path, err := doctor.ReleaseBinary()
	if err != nil {
		return "", err
	}
	if installed, err := doctor.ReleaseVersion(path); err == nil && installed == pin {
		fmt.Fprintf(out, "binary: already release %s%s\n", pin, suffix)
		return "", nil
	}
	if dryRun {
		fmt.Fprintf(out, "binary: fetched release %s%s\n", pin, suffix)
		return "", nil
	}
	base := os.Getenv("KOMODO_RELEASE_URL")
	if base == "" {
		base = releaseURL
	}
	base = strings.TrimSuffix(base, "/") + "/v" + pin
	name := filepath.Base(path)
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	sums, err := fetch(ctx, base+"/"+releaseSums)
	if err != nil {
		return "", err
	}
	want := ""
	for _, entry := range strings.Split(string(sums), "\n") {
		if fields := strings.Fields(entry); len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = fields[0]
		}
	}
	if want == "" {
		return "", fmt.Errorf("%s for release %s lists no %s; nothing was installed", releaseSums, pin, name)
	}
	body, err := fetch(ctx, base+"/"+name)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	if got := hex.EncodeToString(digest[:]); got != want {
		return "", fmt.Errorf("checksum mismatch for %s %s: got %s, want %s; nothing was installed", name, pin, got, want)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	staged := path + ".new"
	if err := os.WriteFile(staged, body, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(staged, path); err != nil {
		return "", err
	}
	fmt.Fprintf(out, "binary: fetched release %s, checksum verified%s\n", pin, suffix)
	return path, nil
}

// httpGet downloads one URL's body, refusing any status but 200 and any body over fetchLimit.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not download %s: %s", url, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, fetchLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > fetchLimit {
		return nil, fmt.Errorf("%s is over %d bytes", url, fetchLimit)
	}
	return body, nil
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
