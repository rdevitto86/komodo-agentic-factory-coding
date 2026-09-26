package backlog

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	groupFileHeading   = regexp.MustCompile(`^##\s+\[(TG-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$`)
	groupFileTaskLine  = regexp.MustCompile(`^-\s+\[([ xX])\]\s+\*\*(TSK-[\w.]+)\*\*\s+(.+?)\s*$`)
	groupFileFieldLine = regexp.MustCompile(`^\s{2}-\s+(files|accept|checks):\s*(.*)$`)
	groupFileBacktick  = regexp.MustCompile("`([^`]+)`")
)

// GroupTask is one checkbox task inside a group file: a title, its files, and optional accept and checks lines.
type GroupTask struct {
	ID     string
	Title  string
	Done   bool
	Files  []string
	Accept []string
	Checks []string
	Line   int
}

// GroupFile is one parsed <group-id>-<slug>.md file: its heading, yaml fields, and its tasks in order.
type GroupFile struct {
	ID        string
	Title     string
	Priority  string
	Status    string
	Type      string
	Version   string
	EpicID    string
	DependsOn []string
	Tasks     []GroupTask
	Problems  []string
}

// ParseGroupFile reads one docs/backlog/<group-id>-<slug>.md file into a GroupFile without judging its content.
func ParseGroupFile(text string) GroupFile {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var file GroupFile
	index := 0
	for index < len(lines) {
		line := lines[index]
		if match := groupFileHeading.FindStringSubmatch(line); match != nil {
			file.ID, file.Title, file.Priority, file.Status = match[1], match[2], match[3], match[4]
			index++
			fields, _, end, err := readBlock(lines, index)
			if err != nil {
				file.Problems = append(file.Problems, err.Error())
			}
			if end >= 0 && err == nil && fields.values != nil {
				file.Type = fields.String("type")
				file.Version = fields.String("version")
				file.EpicID = fields.String("epic")
				file.DependsOn = fields.List("depends_on")
				index = end + 1
			}
			continue
		}
		if match := groupFileTaskLine.FindStringSubmatch(line); match != nil {
			task := GroupTask{
				ID:    match[2],
				Title: strings.TrimSpace(match[3]),
				Done:  strings.ToLower(match[1]) == "x",
				Line:  index,
			}
			index++
			for index < len(lines) {
				fieldMatch := groupFileFieldLine.FindStringSubmatch(lines[index])
				if fieldMatch == nil {
					break
				}
				key, value := fieldMatch[1], strings.TrimSpace(fieldMatch[2])
				switch key {
				case "files":
					task.Files = append(task.Files, splitGroupFileList(value)...)
				case "checks":
					task.Checks = append(task.Checks, splitGroupFileList(value)...)
				case "accept":
					task.Accept = append(task.Accept, value)
				}
				index++
			}
			file.Tasks = append(file.Tasks, task)
			continue
		}
		if file.ID != "" && strings.HasPrefix(strings.TrimSpace(line), "- [") {
			file.Problems = append(file.Problems,
				fmt.Sprintf("line %d: checkbox does not match the task pattern: %q", index+1, line))
		}
		index++
	}
	if file.ID == "" {
		file.Problems = append(file.Problems,
			"no heading matches `## [<group-id>] <title> [P: <letter>] [<STATUS>]`")
	}
	return file
}

// splitGroupFileList splits a comma-separated, optionally backtick-quoted list into plain paths.
func splitGroupFileList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if match := groupFileBacktick.FindStringSubmatch(part); match != nil {
			part = match[1]
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
