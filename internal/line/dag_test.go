package line

import (
	"testing"

	"komodo/internal/backlog"
)

func TestTwoTasksOwningNestedTreesNeverShareAWave(t *testing.T) {
	text := "### [TG-21.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-21.1.1] Wide [P: C] [READY]\n```yaml\nfiles: [internal]\ndone_when: [\"true\"]\n```\n\n" +
		"#### [TSK-21.1.2] Narrow [P: C] [READY]\n```yaml\nfiles: [internal/profile]\ndone_when: [\"true\"]\n```\n"
	parsed := backlog.Parse(text)
	waves, err := Waves(parsed.Groups[0].Tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; a task owning internal cannot run beside one owning internal/profile", waves)
	}
}
