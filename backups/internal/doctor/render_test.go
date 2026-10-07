package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/install"
)

func TestDriftNamesAGlobalRulesFileWhoseBytesDiffer(t *testing.T) {
	home := t.TempDir()
	rules := filepath.Join(home, ".globalhost", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(rules), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := install.Plan{Host: "globalhost", Root: home, Marker: filepath.Join(home, ".globalhost", "rendered"),
		Fix: "komodo install --global"}
	plan.Add(rules, []byte("# Agent Rules\n"), "the universal rules")
	cases := []struct {
		name, onDisk string
		drift        bool
	}{
		{"same bytes", "# Agent Rules\n", false},
		{"hand-edited", "# Agent Rules\nSee BACKLOG.md.\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(rules, []byte(tc.onDisk), 0o644); err != nil {
				t.Fatal(err)
			}
			problems := checkDrift([]renderedHost{{Name: "globalhost global", Plan: plan}}, nil)
			if !tc.drift {
				if len(problems) != 0 {
					t.Fatalf("problems = %+v, want none for matching bytes", problems)
				}
				return
			}
			if len(problems) != 1 || problems[0].Check != checkGlobal || problems[0].Where != rules {
				t.Fatalf("problems = %+v, want %s named as global drift", problems, rules)
			}
		})
	}
}
