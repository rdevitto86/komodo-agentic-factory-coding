package backlog

import "testing"

const importSample = "# Project TODO\n\n" +
	"## Auth\n\n" +
	"- [ ] Add refresh tokens\n" +
	"- [x] Wire up login\n" +
	"* A bullet with no checkbox at all\n" +
	"\n" +
	"A stray paragraph no bullet or heading owns.\n\n" +
	"## Billing\n\n" +
	"- [ ] Refund partial charges\n"

func TestImportOpensAGroupPerHeading(t *testing.T) {
	result := Import(importSample, "TODO.md")
	if len(result.Groups) != 3 {
		t.Fatalf("groups = %d, want 3: %+v", len(result.Groups), result.Groups)
	}
	if result.Groups[0].Title != "Project TODO" || result.Groups[1].Title != "Auth" || result.Groups[2].Title != "Billing" {
		t.Fatalf("titles = %q, %q, %q", result.Groups[0].Title, result.Groups[1].Title, result.Groups[2].Title)
	}
	for _, group := range result.Groups {
		if group.Status != "REFINEMENT" || group.Version != "0.1.0" {
			t.Fatalf("group %+v must open REFINEMENT with a placeholder version", group)
		}
	}
}

func TestImportKeepsATickedTaskTicked(t *testing.T) {
	result := Import(importSample, "TODO.md")
	auth := result.Groups[1]
	if len(auth.Tasks) != 3 {
		t.Fatalf("auth tasks = %d, want 3: %+v", len(auth.Tasks), auth.Tasks)
	}
	if auth.Tasks[0].Done {
		t.Fatal("an open checkbox must not import as done")
	}
	if !auth.Tasks[1].Done {
		t.Fatal("a ticked checkbox must import as done")
	}
	if auth.Tasks[0].Title != "Add refresh tokens" || auth.Tasks[1].Title != "Wire up login" {
		t.Fatalf("titles = %+v", auth.Tasks)
	}
}

func TestImportTakesABareBulletAsATask(t *testing.T) {
	result := Import(importSample, "TODO.md")
	auth := result.Groups[1]
	last := auth.Tasks[len(auth.Tasks)-1]
	if last.Title != "A bullet with no checkbox at all" || last.Done {
		t.Fatalf("last task = %+v, want an open bare-bullet task", last)
	}
}

func TestImportNamesEveryTaskFilesAsItsSource(t *testing.T) {
	result := Import(importSample, "TODO.md")
	for _, group := range result.Groups {
		for _, task := range group.Tasks {
			if len(task.Files) != 1 || task.Files[0] != "TODO.md" {
				t.Fatalf("task %+v files must name the source", task)
			}
		}
	}
}

func TestImportReportsALineItCannotPlaceWithItsNumber(t *testing.T) {
	result := Import(importSample, "TODO.md")
	if len(result.Skipped) != 1 {
		t.Fatalf("skipped = %+v, want 1", result.Skipped)
	}
	if result.Skipped[0].Line != 9 || result.Skipped[0].Text != "A stray paragraph no bullet or heading owns." {
		t.Fatalf("skipped = %+v", result.Skipped[0])
	}
}

func TestImportAssignsEveryGroupAndTaskAUniqueID(t *testing.T) {
	result := Import(importSample, "TODO.md")
	seen := map[string]bool{}
	for _, group := range result.Groups {
		if seen[group.ID] {
			t.Fatalf("duplicate group id %s", group.ID)
		}
		seen[group.ID] = true
		for _, task := range group.Tasks {
			if seen[task.ID] {
				t.Fatalf("duplicate task id %s", task.ID)
			}
			seen[task.ID] = true
		}
	}
}

func TestImportOnATaskOnlyFileOpensAnImplicitGroup(t *testing.T) {
	result := Import("- [ ] Do the thing\n", "TODO.md")
	if len(result.Groups) != 1 || result.Groups[0].Title != "Imported tasks" {
		t.Fatalf("groups = %+v", result.Groups)
	}
	if len(result.Groups[0].Tasks) != 1 {
		t.Fatalf("tasks = %+v", result.Groups[0].Tasks)
	}
}
