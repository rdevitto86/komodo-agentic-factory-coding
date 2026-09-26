package backlog

import (
	"strings"
	"testing"
)

func TestLintRejectsAGroupWithMoreThan12Tasks(t *testing.T) {
	text := "### [TG-40.1] Large group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for i := 1; i <= 13; i++ {
		text += "#### [TSK-40.1." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "] Task " + string(rune('0'+i)) + " [P: C] [READY]\n"
		text += "```yaml\nfiles: [a/t" + string(rune('0'+i)) + ".go]\ndone_when: [\"go test ./...\"]\n```\n\n"
	}
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-40.1") && strings.Contains(problem, "exceeds limit of 12") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions exceeding 12 tasks; got %v", problems)
	}
}

func TestLintAcceptsAGroupWith12Tasks(t *testing.T) {
	text := "### [TG-41.1] Exactly 12 tasks\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for i := 1; i <= 12; i++ {
		text += "#### [TSK-41.1." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "] Task " + string(rune('0'+i)) + " [P: C] [READY]\n"
		text += "```yaml\nfiles: [a/t" + string(rune('0'+i)) + ".go]\ndone_when: [\"go test ./...\"]\n```\n\n"
	}
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "exceeds limit of 12") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("12 tasks should be allowed; got %v", problems)
	}
}

func TestLintRejectsABuildTaskWithLightTier(t *testing.T) {
	text := "### [TG-42.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-42.1.1] Build with light [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: light\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-42.1.1") && strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions build task with light tier; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithStandardTier(t *testing.T) {
	text := "### [TG-43.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-43.1.1] Build with standard [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: standard\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with standard tier should be allowed; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithHeavyTier(t *testing.T) {
	text := "### [TG-44.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-44.1.1] Build with heavy [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: heavy\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with heavy tier should be allowed; got %v", problems)
	}
}

func TestLintRejectsAFeatTaskWithLightTier(t *testing.T) {
	text := "### [TG-45.1] Feat test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-45.1.1] Feat with light [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"tier: light\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-45.1.1") && strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions feat task with light tier; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithNoTier(t *testing.T) {
	text := "### [TG-46.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-46.1.1] Build with no tier [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with no tier should be allowed; got %v", problems)
	}
}
