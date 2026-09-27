package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForModeRunsThreeLensesInFullModeAndOneInEconomy(t *testing.T) {
	cases := []struct {
		mode string
		want []Lens
	}{
		{"full", []Lens{Correctness, Security, Quality}},
		{"", []Lens{Correctness, Security, Quality}},
		{"economy", []Lens{Economy}},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			got := ForMode(tc.mode)
			if len(got) != len(tc.want) {
				t.Fatalf("lenses = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("lenses = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestEveryLensNamesASkillTheToolkitShips(t *testing.T) {
	for _, lens := range []Lens{Correctness, Security, Quality, Economy} {
		path := filepath.Join("..", "..", "komodo", "skills", lens.Skill(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", lens, err)
		}
		if !strings.Contains(string(data), "name: "+lens.Skill()) {
			t.Fatalf("%s names no skill called %s", path, lens.Skill())
		}
	}
}
