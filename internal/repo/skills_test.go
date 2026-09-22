package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// skillsRepo builds a repo root with one SKILL.md per named skill under .komodo/skills.
func skillsRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, SkillsDir, name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadSkillsWithFrontmatterIsNew(t *testing.T) {
	root := skillsRepo(t, map[string]string{
		"local-only": "---\nname: local-only\ndescription: what this repo only knows\n---\n\nBody.\n",
	})
	skills, skipped := LoadSkills(root)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	if len(skills) != 1 || !skills[0].New || skills[0].Name != "local-only" {
		t.Fatalf("skills = %+v", skills)
	}
}

func TestLoadSkillsWithoutFrontmatterAppends(t *testing.T) {
	root := skillsRepo(t, map[string]string{
		"builder": "## Repo overrides\n\nRun `make test` before every commit.\n",
	})
	skills, _ := LoadSkills(root)
	if len(skills) != 1 || skills[0].New {
		t.Fatalf("skills = %+v", skills)
	}
}

func TestLoadSkillsSkipsAProtectedName(t *testing.T) {
	root := skillsRepo(t, map[string]string{
		"run": "## Repo overrides\n\nNever going to happen.\n",
	})
	skills, skipped := LoadSkills(root)
	if len(skills) != 0 {
		t.Fatalf("skills = %+v", skills)
	}
	if len(skipped) != 1 || skipped[0] != "run: run, review, backlog, and respond cannot be appended to" {
		t.Fatalf("skipped = %v", skipped)
	}
}

func TestLoadSkillsWithNoDirReturnsNothing(t *testing.T) {
	root := t.TempDir()
	skills, skipped := LoadSkills(root)
	if skills != nil || skipped != nil {
		t.Fatalf("skills = %v, skipped = %v", skills, skipped)
	}
}
