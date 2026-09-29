package backlog

import (
	"fmt"
	"regexp"
	"strings"
)

// importBullet matches a foreign markdown bullet, plain or with a "[ ]" or "[x]" checkbox.
var importBullet = regexp.MustCompile(`^[-*+]\s+(?:\[([ xX])\]\s+)?(.+)$`)

// importHeading matches a foreign markdown heading of any level.
var importHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

// ImportSkip is one line Import could not place under a group or as a task.
type ImportSkip struct {
	Line int
	Text string
}

// ImportResult is every REFINEMENT group Import placed, and every line it could not.
type ImportResult struct {
	Groups  []GroupFile
	Skipped []ImportSkip
}

// Import turns a foreign TODO.md or free-form BACKLOG.md into REFINEMENT group files: a heading
// opens a group, a bullet becomes a task, ticked stays ticked; every task's files name source.
func Import(text, source string) ImportResult {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var result ImportResult
	var current *GroupFile
	groupIndex := 0
	openGroup := func(title string) {
		if current != nil {
			result.Groups = append(result.Groups, *current)
		}
		groupIndex++
		current = &GroupFile{
			ID: fmt.Sprintf("TG-IMPORT.%d", groupIndex), Title: title,
			Priority: "M", Status: "REFINEMENT", Type: "chore", Version: "0.1.0",
		}
	}
	for index, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if match := importHeading.FindStringSubmatch(trimmed); match != nil {
			openGroup(match[2])
			continue
		}
		if match := importBullet.FindStringSubmatch(trimmed); match != nil {
			if current == nil {
				openGroup("Imported tasks")
			}
			done := strings.EqualFold(match[1], "x")
			taskID := fmt.Sprintf("TSK-IMPORT.%d.%d", groupIndex, len(current.Tasks)+1)
			current.Tasks = append(current.Tasks, GroupTask{
				ID: taskID, Title: strings.TrimSpace(match[2]), Done: done, Files: []string{source},
			})
			continue
		}
		result.Skipped = append(result.Skipped, ImportSkip{Line: index + 1, Text: raw})
	}
	if current != nil {
		result.Groups = append(result.Groups, *current)
	}
	return result
}
