package review

import "testing"

// repairDiff is a repair that rewrote a.go's line 4 and added b.go's lines 1 and 2.
const repairDiff = "--- a/a.go\n+++ b/a.go\n@@ -3,3 +3,3 @@\n func A() {\n-\treturn\n+\tpanic(nil)\n }\n" +
	"--- /dev/null\n+++ b/b.go\n@@ -0,0 +1,2 @@\n+package a\n+func B() {}\n"

// TestARereviewDropsANewFindingOnAnUnchangedLine: a resumed lens may only keep its own
// open findings or flag a line the repair changed.
func TestARereviewDropsANewFindingOnAnUnchangedLine(t *testing.T) {
	open := []Finding{{Lens: "correctness", File: "a.go", Line: 9, Title: "nil map"}}
	cases := []struct {
		name    string
		finding Finding
		kept    bool
	}{
		{"a new finding on an unchanged line", Finding{File: "a.go", Line: 3, Title: "widened"}, false},
		{"a new finding in a file the repair left alone", Finding{File: "c.go", Line: 4, Title: "widened"}, false},
		{"an open finding kept", Finding{File: "a.go", Line: 9, Title: "nil map"}, true},
		{"a new finding on a line the repair rewrote", Finding{File: "a.go", Line: 4, Title: "panics"}, true},
		{"a new finding in a file the repair added", Finding{File: "b.go", Line: 2, Title: "undocumented"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Rereview(open, []Finding{tc.finding}, repairDiff)
			if kept := len(got) == 1; kept != tc.kept {
				t.Fatalf("Rereview kept %+v, want kept = %v", got, tc.kept)
			}
		})
	}
}

func TestARereviewThatClosesEveryFindingKeepsNone(t *testing.T) {
	open := []Finding{{File: "a.go", Line: 9}}
	got := Rereview(open, nil, repairDiff)
	if got == nil || len(got) != 0 {
		t.Fatalf("Rereview = %#v, want an empty, non-nil list", got)
	}
}
