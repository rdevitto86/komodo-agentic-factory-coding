package main

import (
	"os"
	"strings"
	"testing"
)

// TestTheKomodoSkillMatchesTheHelp fails the gate when the shipped skill drifts from the binary's usage.
func TestTheKomodoSkillMatchesTheHelp(t *testing.T) {
	shipped, err := os.ReadFile("../../komodo/skills/komodo/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(shipped) != helpSkill() {
		t.Fatal("komodo/skills/komodo/SKILL.md drifted from komodo help; " +
			"run go run ./cmd/komodo help --skill > komodo/skills/komodo/SKILL.md")
	}
}

// TestHelpPrintsTheUsageOrTheSkillOutsideARepo proves help needs no repo, and --skill carries every command.
func TestHelpPrintsTheUsageOrTheSkillOutsideARepo(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"help"}, "komodo: the code assembly line."},
		{[]string{"--help"}, "komodo lint"},
		{[]string{"help", "--skill"}, "name: komodo\n"},
	}
	for _, each := range cases {
		t.Run(strings.Join(each.args, " "), func(t *testing.T) {
			got := runCLI(t, t.TempDir(), "", each.args...)
			if got.code != 0 || !strings.Contains(got.stdout, each.want) {
				t.Fatalf("komodo %v exited %d, printed %q; want %q", each.args, got.code, got.stdout, each.want)
			}
		})
	}
	skill := runCLI(t, t.TempDir(), "", "help", "--skill").stdout
	for _, line := range strings.Split(usage, "\n") {
		if strings.HasPrefix(line, "  komodo ") && !strings.Contains(skill, line) {
			t.Fatalf("the skill leaves out %q", line)
		}
	}
}
