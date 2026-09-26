package backlog

import "testing"

const groupFileSample = "## [TG-04.1] Tokens report their expiry [P: H] [READY]\n\n" +
	"```yaml\ntype: feat\nversion: 1.4.0\nepic: EPIC-04\ndepends_on: []\n```\n\n" +
	"- [ ] **TSK-04.1.1** Tokens know when they expire\n" +
	"  - files: `internal/token/`\n" +
	"  - accept: Expired is true at or after ExpiresAt\n" +
	"- [x] **TSK-04.1.2** An expired token gets a 401\n" +
	"  - files: `internal/api/auth.go`, `internal/api/auth_test.go`\n"

func TestParseGroupFileReadsHeadingAndFields(t *testing.T) {
	file := ParseGroupFile(groupFileSample)
	if file.ID != "TG-04.1" || file.Title != "Tokens report their expiry" {
		t.Fatalf("file = %+v", file)
	}
	if file.Priority != "H" || file.Status != "READY" {
		t.Fatalf("priority/status = %q/%q", file.Priority, file.Status)
	}
	if file.Type != "feat" || file.Version != "1.4.0" || file.EpicID != "EPIC-04" {
		t.Fatalf("type/version/epic = %q/%q/%q", file.Type, file.Version, file.EpicID)
	}
	if len(file.DependsOn) != 0 {
		t.Fatalf("depends_on = %v, want empty", file.DependsOn)
	}
	if len(file.Problems) != 0 {
		t.Fatalf("problems = %v", file.Problems)
	}
}

func TestParseGroupFileReadsTasks(t *testing.T) {
	file := ParseGroupFile(groupFileSample)
	if len(file.Tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(file.Tasks))
	}
	first := file.Tasks[0]
	if first.ID != "TSK-04.1.1" || first.Title != "Tokens know when they expire" || first.Done {
		t.Fatalf("first = %+v", first)
	}
	if got := first.Files; len(got) != 1 || got[0] != "internal/token/" {
		t.Fatalf("files = %v", got)
	}
	if got := first.Accept; len(got) != 1 || got[0] != "Expired is true at or after ExpiresAt" {
		t.Fatalf("accept = %v", got)
	}
	second := file.Tasks[1]
	if !second.Done {
		t.Fatal("second task must be done")
	}
	if got := second.Files; len(got) != 2 || got[0] != "internal/api/auth.go" || got[1] != "internal/api/auth_test.go" {
		t.Fatalf("files = %v", got)
	}
}

func TestParseGroupFileAcceptsATaskWithOnlyTitleAndFiles(t *testing.T) {
	text := "## [TG-05.1] Bare group [P: M] [READY]\n\n```yaml\ntype: chore\nversion: 1.0.0\nepic: EPIC-05\n" +
		"depends_on: []\n```\n\n- [ ] **TSK-05.1.1** Only a title and files\n  - files: `a.go`\n"
	file := ParseGroupFile(text)
	if len(file.Problems) != 0 {
		t.Fatalf("problems = %v", file.Problems)
	}
	if len(file.Tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(file.Tasks))
	}
	task := file.Tasks[0]
	if len(task.Accept) != 0 || len(task.Checks) != 0 {
		t.Fatalf("task = %+v, want no accept or checks", task)
	}
}

func TestParseGroupFileReportsAMalformedCheckbox(t *testing.T) {
	text := "## [TG-06.1] Broken [P: L] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-06\n" +
		"depends_on: []\n```\n\n- [?] not a real checkbox\n"
	file := ParseGroupFile(text)
	if len(file.Problems) == 0 {
		t.Fatal("want a problem for the malformed checkbox")
	}
}

func TestParseGroupFileReportsAnUnterminatedBlock(t *testing.T) {
	file := ParseGroupFile("## [TG-07.1] Bad [P: C] [READY]\n\n```yaml\ntype: feat\n")
	if len(file.Problems) == 0 {
		t.Fatal("want a problem for the unterminated yaml block")
	}
}
