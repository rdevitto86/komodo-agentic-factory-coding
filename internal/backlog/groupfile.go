package backlog

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	groupFileHeading   = regexp.MustCompile(`^##\s+\[(TG-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$`)
	groupFileTaskLine  = regexp.MustCompile(`^-\s+\[([ xX])\]\s+\*\*(TSK-[\w.]+)\*\*\s+(.+?)\s*$`)
	groupFileFieldLine = regexp.MustCompile(`^\s{2}-\s+(files|accept|done_when|checks|owner|context|depends_on|priority|status|tier|facets):\s*(.*)$`)
)

// GroupTask is one checkbox task inside a group file, with its optional owner, context, depends_on, priority and status.
type GroupTask struct {
	ID        string
	Title     string
	Done      bool
	Files     []string
	Accept    []string
	Checks    []string
	Owner     string
	Context   []string
	DependsOn []string
	Priority  string
	Status    string
	Tier      string
	Facets    []string
	Line      int
}

// GroupFile is one group as its index and task files assemble: heading, yaml fields, and tasks in order.
type GroupFile struct {
	ID        string
	Title     string
	Priority  string
	Status    string
	Type      string
	Version   string
	EpicID    string
	Mode      string
	Base      string
	DependsOn []string
	Tasks     []GroupTask
	Problems  []string
}

// ParseGroupFile reads group grammar text, a group index file with its task files appended, without judging its content.
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
				file.Mode = fields.String("mode")
				file.Base = fields.String("base")
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
				case "done_when", "checks":
					task.Checks = append(task.Checks, splitGroupFileList(value)...)
				case "accept":
					task.Accept = append(task.Accept, value)
				case "owner":
					task.Owner = value
				case "context":
					task.Context = append(task.Context, splitGroupFileList(value)...)
				case "depends_on":
					task.DependsOn = append(task.DependsOn, splitGroupFileList(value)...)
				case "priority":
					task.Priority = value
				case "status":
					task.Status = strings.ToUpper(value)
				case "tier":
					task.Tier = value
				case "facets":
					task.Facets = append(task.Facets, splitGroupFileList(value)...)
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

// RenderGroupFileTask renders one checkbox task line and its fields, in group-file grammar order.
func RenderGroupFileTask(task GroupTask) string {
	check := " "
	if task.Done {
		check = "x"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- [%s] **%s** %s\n", check, task.ID, strings.TrimSpace(task.Title))
	if len(task.Files) > 0 {
		fmt.Fprintf(&b, "  - files: %s\n", strings.Join(backtickEach(task.Files), ", "))
	}
	for _, line := range task.Accept {
		fmt.Fprintf(&b, "  - accept: %s\n", line)
	}
	if len(task.Checks) > 0 {
		fmt.Fprintf(&b, "  - done_when: %s\n", strings.Join(backtickEach(task.Checks), ", "))
	}
	if task.Owner != "" {
		fmt.Fprintf(&b, "  - owner: %s\n", task.Owner)
	}
	if len(task.Context) > 0 {
		fmt.Fprintf(&b, "  - context: %s\n", strings.Join(backtickEach(task.Context), ", "))
	}
	if len(task.DependsOn) > 0 {
		fmt.Fprintf(&b, "  - depends_on: %s\n", strings.Join(task.DependsOn, ", "))
	}
	if task.Priority != "" {
		fmt.Fprintf(&b, "  - priority: %s\n", task.Priority)
	}
	if task.Status != "" {
		fmt.Fprintf(&b, "  - status: %s\n", task.Status)
	}
	if task.Tier != "" {
		fmt.Fprintf(&b, "  - tier: %s\n", task.Tier)
	}
	if len(task.Facets) > 0 {
		fmt.Fprintf(&b, "  - facets: %s\n", strings.Join(task.Facets, ", "))
	}
	return b.String()
}

// RenderGroupFileDocument renders one full group file: its heading, yaml block, then every task in order.
func RenderGroupFileDocument(file GroupFile) string {
	var fields Fields
	fields.Set("type", file.Type)
	fields.Set("version", file.Version)
	fields.Set("epic", file.EpicID)
	if file.Mode != "" {
		fields.Set("mode", file.Mode)
	}
	if file.Base != "" {
		fields.Set("base", file.Base)
	}
	fields.Set("depends_on", toAnyList(file.DependsOn))
	out := RenderGroupFile(file.ID, file.Title, file.Priority, file.Status, fields)
	for _, task := range file.Tasks {
		out += "\n" + RenderGroupFileTask(task)
	}
	return out
}

// splitGroupFileList splits a comma-separated list into items; a backtick-quoted item keeps its commas.
func splitGroupFileList(value string) []string {
	var out []string
	rest := value
	for rest != "" {
		rest = strings.TrimLeft(rest, " \t,")
		if rest == "" {
			break
		}
		if rest[0] == '`' {
			end := strings.IndexByte(rest[1:], '`')
			if end >= 0 {
				if item := strings.TrimSpace(rest[1 : end+1]); item != "" {
					out = append(out, item)
				}
				rest = rest[end+2:]
				continue
			}
		}
		next := strings.IndexByte(rest, ',')
		if next < 0 {
			next = len(rest)
		}
		if item := strings.TrimSpace(rest[:next]); item != "" {
			out = append(out, item)
		}
		rest = rest[next:]
	}
	return out
}
