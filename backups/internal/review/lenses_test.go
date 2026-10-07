package review

import (
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
