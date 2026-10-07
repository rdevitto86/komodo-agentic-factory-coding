package guard

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/mount"
)

// registerFakeWriteHost registers testhost with one extra path a session may write outside root.
func registerFakeWriteHost(writePaths func(root string) []string) {
	mount.Register(mount.Host{Name: "testhost", ConfigPaths: []string{"~/.testhost/**"}, WritePaths: writePaths})
}

func TestAHostsDeclaredWritePathIsAllowedOutsideTheWorktree(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	memory := filepath.Join(filepath.Dir(outsideTemp(t)), "memory")
	registerFakeWriteHost(func(string) []string { return []string{filepath.ToSlash(memory) + "/**"} })
	target := filepath.Join(memory, "note.md")
	if findings := pathFindings(target, root, root, DefaultPolicy()); len(findings) != 0 {
		t.Fatalf("a declared write path was refused: %v", findings)
	}
}

func TestAPathOutsideEveryDeclaredWritePathIsStillRefused(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	root := outsideTemp(t)
	registerFakeWriteHost(func(string) []string { return nil })
	target := filepath.Join(filepath.Dir(root), "elsewhere.md")
	if findings := pathFindings(target, root, root, DefaultPolicy()); len(findings) == 0 {
		t.Fatal("a write outside the worktree with no declared write path passed")
	}
}

// TestADollarHomePathExpandsAndIsRefused proves a $HOME reference expands against the session's
// environment and is judged as the real config path it names, not skipped outright.
func TestADollarHomePathExpandsAndIsRefused(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := worktree(t)
	policy := DefaultPolicy()
	policy.ConfigPaths = append(policy.ConfigPaths, "~/.claude/**")
	if findings := pathFindings("$HOME/.claude/CLAUDE.md", root, root, policy); len(findings) == 0 {
		t.Fatal("a $HOME config path expanded past the guard")
	}
}

// TestABracedHomePathExpandsAndIsRefused proves the ${VAR} form expands the same way $VAR does.
func TestABracedHomePathExpandsAndIsRefused(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := worktree(t)
	policy := DefaultPolicy()
	policy.ConfigPaths = append(policy.ConfigPaths, "~/.claude/**")
	if findings := pathFindings("${HOME}/.claude/settings.json", root, root, policy); len(findings) == 0 {
		t.Fatal("a ${HOME} config path expanded past the guard")
	}
}

// TestAPathWithCommandSubstitutionStaysRefused proves a path the guard cannot resolve, since it
// carries a command substitution, is still judged on its literal text, never allowed outright.
func TestAPathWithCommandSubstitutionStaysRefused(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := worktree(t)
	policy := DefaultPolicy()
	policy.ConfigPaths = append(policy.ConfigPaths, "~/.claude/**")
	if findings := pathFindings("~/.claude/x$(true)", root, root, policy); len(findings) == 0 {
		t.Fatal("a path with a command substitution was allowed past the guard")
	}
}

// TestABacktickPathIsStillJudgedNormally proves a path naming a backtick is never blanket-skipped;
// one that genuinely sits outside the worktree is still refused.
func TestABacktickPathIsStillJudgedNormally(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	if findings := pathFindings("/etc/passwd`", root, root, DefaultPolicy()); len(findings) == 0 {
		t.Fatal("a backtick-carrying path outside the worktree was allowed")
	}
}

// TestAPathNamingTMPDIRExpandsAndPasses proves the logged false positive is fixed by expansion:
// once $TMPDIR resolves, the path lands inside the real temp directory isAllowedWrite already covers.
func TestAPathNamingTMPDIRExpandsAndPasses(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	t.Setenv("TMPDIR", os.TempDir())
	if findings := pathFindings("/tmp/../$TMPDIR/v.log", root, root, DefaultPolicy()); len(findings) != 0 {
		t.Fatalf("a path naming TMPDIR was refused: %v", findings)
	}
}

func TestAConfigPathStillWinsOverADeclaredWritePath(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := worktree(t)
	memory := filepath.Join(home, ".testhost", "memory")
	registerFakeWriteHost(func(string) []string { return []string{filepath.ToSlash(memory) + "/**"} })
	config := filepath.Join(home, ".testhost", "settings.json")
	policy := DefaultPolicy()
	policy.ConfigPaths = append(policy.ConfigPaths, "~/.testhost/settings.json")
	if findings := pathFindings(config, root, root, policy); len(findings) == 0 {
		t.Fatal("a host's own config path was allowed as a declared write path")
	}
	if findings := pathFindings(filepath.Join(memory, "note.md"), root, root, policy); len(findings) != 0 {
		t.Fatalf("the declared memory path was still refused: %v", findings)
	}
}

// TestAWriteBesideAWorktreeInTempIsRefusedOnEveryPlatform proves a worktree that itself sits in the
// temp directory gains no sibling writes, whichever spelling of the temp path the root or write uses.
func TestAWriteBesideAWorktreeInTempIsRefusedOnEveryPlatform(t *testing.T) {
	t.Setenv(RoleEnv, "builder")
	parent := t.TempDir()
	for _, root := range []string{filepath.Join(parent, "wt"), realPath(filepath.Join(parent, "wt"))} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if findings := pathFindings("../outside.txt", root, root, DefaultPolicy()); len(findings) == 0 {
			t.Fatalf("a write beside %s was allowed", root)
		}
		scratch := filepath.Join(os.TempDir(), "komodo-scratch.txt")
		if findings := pathFindings(scratch, root, root, DefaultPolicy()); len(findings) != 0 {
			t.Fatalf("scratch %s = %v, want a temp file away from the worktree allowed", scratch, findings)
		}
	}
}

func TestRealPathResolvesTheLongestExistingPrefix(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := realPath(filepath.Join(dir, "missing", "file.go")), filepath.Join(resolved, "missing", "file.go"); got != want {
		t.Fatalf("realPath = %s, want %s", got, want)
	}
}
