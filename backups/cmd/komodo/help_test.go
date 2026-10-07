package main

import (
	"os"
	"path/filepath"
	"regexp"
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
		{[]string{"help"}, "komodo: the coding harness."},
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

// docCommand is a backticked komodo invocation in prose: the subcommand, then the rest of the span.
var docCommand = regexp.MustCompile("`komodo ([a-z][a-z-]*)([^`]*)`")

// docFlag is one long flag inside a documented invocation.
var docFlag = regexp.MustCompile(`--([a-z][a-z-]*)`)

// TestDocCommandsNameRealSubcommandsAndFlags proves every komodo command the README, the design and the
// markdown a model reads names a subcommand the binary has, with flags some subcommand defines.
func TestDocCommandsNameRealSubcommandsAndFlags(t *testing.T) {
	subcommands := map[string]bool{}
	for _, line := range strings.Split(usage, "\n") {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "komodo" {
			subcommands[fields[1]] = true
		}
	}
	var source strings.Builder
	goFiles, _ := filepath.Glob("*.go")
	for _, path := range goFiles {
		if data, err := os.ReadFile(path); err == nil {
			source.Write(data)
		}
	}
	docs := []string{filepath.Join("..", "..", "README.md"), filepath.Join("..", "..", "docs", "lld.md")}
	_ = filepath.WalkDir(filepath.Join("..", "..", "komodo"), func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".md") {
			docs = append(docs, path)
		}
		return nil
	})
	for _, doc := range docs {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range docCommand.FindAllStringSubmatch(string(data), -1) {
			if !subcommands[match[1]] {
				t.Errorf("%s names `komodo %s`, which the binary does not have", doc, match[1])
				continue
			}
			for _, flag := range docFlag.FindAllStringSubmatch(match[2], -1) {
				defined := regexp.MustCompile(`\.(Bool|String|Int|Int64|Uint|Duration|Float64|Func)\("` + regexp.QuoteMeta(flag[1]) + `"|\.Var\(&\w+, "` + regexp.QuoteMeta(flag[1]) + `"`)
				if !defined.MatchString(source.String()) {
					t.Errorf("%s names `komodo %s --%s`, a flag no subcommand defines", doc, match[1], flag[1])
				}
			}
		}
	}
}
