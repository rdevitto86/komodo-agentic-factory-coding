package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// toolkit builds a root holding the rules, one role, and one skill.
func toolkit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("komodo/AGENTS.md", "# Agent Rules\n\n## Git\n\n- Free inside the worktree.\n\n{{accessibility}}\n")
	write("komodo/rules/accessibility.md", "## Writing for a human\n- **Answer first.**\n")
	write("komodo/roles/builder.md", "---\nname: builder\ndescription: Writes code.\ntier: standard\n"+
		"tools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\n"+
		"You are a builder.\n\n# Brief\n\nTask {{task_id}}\n")
	write("komodo/roles/summarizer.md", "---\nname: summarizer\ndescription: Compresses text.\ntier: light\n"+
		"tools: []\nsession: false\nreturns: summarizer.schema.json\n---\n\nYou compress text.\n")
	write("komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\n---\n\n# Go\n")
	return root
}

func TestLoadRolesReadsEveryFrontmatterKey(t *testing.T) {
	roles, err := LoadRoles(toolkit(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 || roles[0].Name != "builder" {
		t.Fatalf("roles = %+v", roles)
	}
	builder := roles[0]
	if builder.Tier != "standard" || !builder.Session || builder.Returns != "builder.schema.json" {
		t.Fatalf("builder = %+v", builder)
	}
	if len(builder.Tools) != 5 || builder.Tools[0] != "read" {
		t.Fatalf("tools = %v", builder.Tools)
	}
}

func TestInstructionsDropTheBriefTemplate(t *testing.T) {
	roles, _ := LoadRoles(toolkit(t))
	text := roles[0].Instructions()
	if !strings.Contains(text, "You are a builder") || strings.Contains(text, "{{task_id}}") {
		t.Fatalf("instructions = %q", text)
	}
}

func TestWritesFollowsTheVerbs(t *testing.T) {
	roles, _ := LoadRoles(toolkit(t))
	if !roles[0].Writes() {
		t.Fatal("a role with write and shell does not write")
	}
	if roles[1].Writes() {
		t.Fatal("a role with no tools writes")
	}
}

func TestRulesFillTheAccessibilitySlot(t *testing.T) {
	text, err := Rules(toolkit(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "{{accessibility}}") || !strings.Contains(text, "Answer first") {
		t.Fatalf("rules = %q", text)
	}
}

func TestVerbsAreTheFive(t *testing.T) {
	if strings.Join(Verbs, ",") != "read,edit,write,shell,search" {
		t.Fatalf("verbs = %v", Verbs)
	}
}

func TestLoadSkillsToleratesNoSkills(t *testing.T) {
	skills, err := LoadSkills(t.TempDir())
	if err != nil || skills != nil {
		t.Fatalf("skills = %v, err = %v", skills, err)
	}
}
