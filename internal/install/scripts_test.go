package install

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scriptRepo commits files into a fresh repo and returns its root.
func scriptRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		writeFile(t, filepath.Join(root, path), body, 0o644)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root
}

func TestScriptProblemsAllowsTrampolinesAndLeanInstallersOnly(t *testing.T) {
	root := scriptRepo(t, map[string]string{
		"hook.sh":     "#!/bin/sh\n# one exec\nexec komodo git-hook pre-commit \"$@\"\n",
		"build.sh":    "#!/bin/sh\ngo vet ./...\ngo test ./...\n",
		"install.sh":  "#!/bin/sh\nset -eu\n[ \"$a\" = \"$b\" ] || { echo \"checksum mismatch\" >&2; exit 1; }\nif [ -x x ]; then echo y; fi\n",
		"install.ps1": strings.Repeat("Write-Host x\n", 21),
	})
	got := strings.Join(ScriptProblems(root), "\n")
	for _, want := range []string{"build.sh: 2 commands", "install.sh: branches", "install.ps1: 21 lines"} {
		if !strings.Contains(got, want) {
			t.Fatalf("problems = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "hook.sh") {
		t.Fatalf("problems = %q; a one-line trampoline is allowed", got)
	}
}

func TestSkillProblemsRefusesAChainedCommandOutsideStandards(t *testing.T) {
	root := scriptRepo(t, map[string]string{
		"komodo/skills/run/SKILL.md":             "# Run\n\n```\nkomodo step\ngit log | head\n```\n",
		"komodo/skills/standards-shell/SKILL.md": "```sh\nfind . | sort\n```\n",
	})
	got := SkillProblems(root)
	if len(got) != 1 || !strings.Contains(got[0], "komodo/skills/run/SKILL.md:5") {
		t.Fatalf("problems = %q, want only the run skill's piped line", got)
	}
}
