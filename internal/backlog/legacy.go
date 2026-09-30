package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	epicHeading  = regexp.MustCompile(`^##\s+\[(EPIC-[\w.]+)\]\s*(.*?)\s*$`)
	groupHeading = regexp.MustCompile(`^###\s+\[(TG-[\w.]+)\]\s*(.*?)\s*$`)
	taskHeading  = regexp.MustCompile(`^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$`)
	taskLike     = regexp.MustCompile(`^####\s+\[TSK-`)
)

var legacyStatus = map[string]string{"WIP": "IN_PROGRESS", "TODO": "READY"}

// LegacyName is the file name Find and doctor look for, the one legacy grammar `komodo migrate` converts.
const LegacyName = "BACKLOG.md"

// Find locates a legacy backlog file at the repo root or under docs/, the input `komodo migrate` converts.
func Find(root string) (string, error) {
	for _, candidate := range []string{LegacyName, filepath.Join("docs", LegacyName)} {
		path := filepath.Join(root, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("no legacy backlog file at %s or %s/docs", root, root)
}

// Parse reads a legacy backlog file's text into groups and tasks without judging their content.
func Parse(text string) Backlog {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	parsed := Backlog{Lines: lines}
	epicID := ""
	current := -1
	index := 0
	for index < len(lines) {
		line := lines[index]
		if match := epicHeading.FindStringSubmatch(line); match != nil {
			epicID = match[1]
			epicTitle := match[2]
			// Look ahead for the goal line containing "Ships as" information
			for i := index + 1; i < len(lines) && i < index+5; i++ {
				nextLine := strings.TrimSpace(lines[i])
				if nextLine == "" {
					continue
				}
				if strings.HasPrefix(nextLine, "#") || strings.HasPrefix(nextLine, "###") {
					break // Stop at next heading
				}
				if idx := strings.Index(nextLine, "Ships as `"); idx >= 0 {
					epicTitle = epicTitle + " " + nextLine
					break
				}
			}
			epic := Epic{ID: epicID, Title: epicTitle}
			parsed.Epics = append(parsed.Epics, epic)
			index++
			continue
		}
		if match := groupHeading.FindStringSubmatch(line); match != nil {
			group := Group{ID: match[1], Title: match[2], Heading: index, EpicID: epicID}
			fields, _, end, err := readBlock(lines, index+1)
			if err != nil {
				parsed.Problems = append(parsed.Problems, err.Error())
			}
			if end >= 0 && err == nil && fields.values != nil {
				group.Fields = fields
				index = end + 1
			} else {
				index++
			}
			parsed.Groups = append(parsed.Groups, group)
			current = len(parsed.Groups) - 1
			continue
		}
		if match := taskHeading.FindStringSubmatch(line); match != nil {
			status := strings.ToUpper(match[4])
			if mapped, ok := legacyStatus[status]; ok {
				status = mapped
			}
			task := Task{
				ID: match[1], Title: strings.TrimSpace(match[2]), Priority: match[3],
				Status: status, Heading: index, BlockStart: -1, BlockEnd: -1,
			}
			fields, start, end, err := readBlock(lines, index+1)
			if err != nil {
				parsed.Problems = append(parsed.Problems, err.Error())
			}
			if end >= 0 && err == nil && fields.values != nil {
				task.Fields = fields
				task.BlockStart, task.BlockEnd = start, end
				index = end + 1
			} else {
				index++
			}
			if current < 0 {
				parsed.Problems = append(parsed.Problems,
					fmt.Sprintf("line %d: task %s appears before any ### [TG-] heading", task.Heading+1, task.ID))
				parsed.Groups = append(parsed.Groups, Group{ID: "TG-ORPHAN", Title: "Orphaned tasks", Heading: task.Heading})
				current = len(parsed.Groups) - 1
			}
			task.GroupID = parsed.Groups[current].ID
			parsed.Groups[current].Tasks = append(parsed.Groups[current].Tasks, task)
			continue
		}
		if taskLike.MatchString(line) {
			parsed.Problems = append(parsed.Problems,
				fmt.Sprintf("line %d: heading does not match the task pattern: %q", index+1, line))
		}
		index++
	}
	return parsed
}

// Load reads and parses one legacy backlog file.
func Load(path string) (Backlog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Backlog{}, err
	}
	return Parse(string(data)), nil
}
