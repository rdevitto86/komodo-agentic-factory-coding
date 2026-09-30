package guard

import (
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
