package backlog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var statusToken = regexp.MustCompile(`\[[A-Z_]+\]\s*$`)

// groupFileCheckboxPrefix matches a group file task's checkbox, open or ticked.
var groupFileCheckboxPrefix = regexp.MustCompile(`^-\s+\[[ xX]\]`)

// groupFileStatusFieldLine matches a group file task's own status override field.
var groupFileStatusFieldLine = regexp.MustCompile(`^\s{2}-\s+status:\s*.*$`)

// SetStatus rewrites one task heading's status token, leaving every other byte alone.
func SetStatus(text, taskID, status string) (string, error) {
	if !contains(Statuses, status) {
		return "", fmt.Errorf("unknown status %q", status)
	}
	lines := strings.SplitAfter(text, "\n")
	for index, line := range lines {
		bare := strings.TrimRight(line, "\r\n")
		match := taskHeading.FindStringSubmatch(bare)
		if match == nil || match[1] != taskID {
			continue
		}
		lines[index] = statusToken.ReplaceAllString(bare, "["+status+"]") + line[len(bare):]
		return strings.Join(lines, ""), nil
	}
	return "", fmt.Errorf("task %s not found", taskID)
}

// SetGroupFileTaskStatus writes a group file task's tick or blocker: DONE flips its checkbox, any
// other status sets or replaces its own status field, leaving every other byte alone.
func SetGroupFileTaskStatus(text, taskID, status string) (string, error) {
	if !contains(Statuses, status) {
		return "", fmt.Errorf("unknown status %q", status)
	}
	file := ParseGroupFile(text)
	var task *GroupTask
	for index := range file.Tasks {
		if file.Tasks[index].ID == taskID {
			task = &file.Tasks[index]
			break
		}
	}
	if task == nil {
		return "", fmt.Errorf("task %s not found", taskID)
	}
	lines := strings.SplitAfter(text, "\n")
	if status == "DONE" {
		lines[task.Line] = groupFileCheckboxPrefix.ReplaceAllString(lines[task.Line], "- [x]")
		return strings.Join(lines, ""), nil
	}
	end := task.Line + 1
	for end < len(lines) && groupFileFieldLine.MatchString(strings.TrimRight(lines[end], "\r\n")) {
		end++
	}
	statusLine := fmt.Sprintf("  - status: %s\n", status)
	for index := task.Line + 1; index < end; index++ {
		if groupFileStatusFieldLine.MatchString(strings.TrimRight(lines[index], "\r\n")) {
			lines[index] = statusLine
			return strings.Join(lines, ""), nil
		}
	}
	out := append([]string{}, lines[:end]...)
	out = append(out, statusLine)
	out = append(out, lines[end:]...)
	return strings.Join(out, ""), nil
}

// RenderTask renders a heading plus its fenced block in the grammar.
func RenderTask(taskID, title, priority, status string, fields Fields) string {
	heading := fmt.Sprintf("#### [%s] %s [P: %s] [%s]", taskID, strings.TrimSpace(title), priority, status)
	return heading + "\n```yaml\n" + DumpFields(fields) + "```\n"
}

// NextTaskID is the next free task id under a group, counting from the highest existing suffix.
func NextTaskID(group Group) string {
	base := strings.TrimPrefix(group.ID, "TG-")
	highest := 0
	for _, task := range group.Tasks {
		parts := strings.Split(task.ID, ".")
		suffix, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil && suffix > highest {
			highest = suffix
		}
	}
	return fmt.Sprintf("TSK-%s.%d", base, highest+1)
}

// groupEnd is the first line after heading that opens the next group or epic, or a rule between them.
func groupEnd(lines []string, heading int) int {
	for index := heading + 1; index < len(lines); index++ {
		line := strings.TrimRight(lines[index], "\r\n")
		if strings.TrimSpace(line) == "---" || epicHeading.MatchString(line) || groupHeading.MatchString(line) {
			return index
		}
	}
	return len(lines)
}

// RenderGroupFile renders a fresh <group-id>-<slug>.md file: its heading and yaml block, with no tasks yet.
func RenderGroupFile(id, title, priority, status string, fields Fields) string {
	heading := fmt.Sprintf("## [%s] %s [P: %s] [%s]", id, strings.TrimSpace(title), priority, status)
	return heading + "\n\n```yaml\n" + DumpFields(fields) + "```\n"
}

// NextGroupFileTaskID is the next free task id in a group file, counting from the highest existing suffix.
func NextGroupFileTaskID(file GroupFile) string {
	base := strings.TrimPrefix(file.ID, "TG-")
	highest := 0
	for _, task := range file.Tasks {
		parts := strings.Split(task.ID, ".")
		suffix, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil && suffix > highest {
			highest = suffix
		}
	}
	return fmt.Sprintf("TSK-%s.%d", base, highest+1)
}

// AppendGroupFileTask adds a checkbox task at the end of a group file's text, returning the text and its id.
func AppendGroupFileTask(text, title string, files, accept []string) (string, string, error) {
	file := ParseGroupFile(text)
	if file.ID == "" {
		return "", "", fmt.Errorf("no group heading found")
	}
	if len(files) == 0 {
		return "", "", fmt.Errorf("task declares no files")
	}
	taskID := NextGroupFileTaskID(file)
	var block strings.Builder
	fmt.Fprintf(&block, "- [ ] **%s** %s\n", taskID, strings.TrimSpace(title))
	fmt.Fprintf(&block, "  - files: %s\n", strings.Join(backtickEach(files), ", "))
	for _, line := range accept {
		fmt.Fprintf(&block, "  - accept: %s\n", line)
	}
	out := strings.TrimRight(text, "\n") + "\n" + block.String()
	return out, taskID, nil
}

// AppendGroupFileTaskWith adds a checkbox task at the end of a group file's text, using every field
// task already carries except its id, which is assigned next in sequence.
func AppendGroupFileTaskWith(text string, task GroupTask) (string, string, error) {
	file := ParseGroupFile(text)
	if file.ID == "" {
		return "", "", fmt.Errorf("no group heading found")
	}
	task.ID = NextGroupFileTaskID(file)
	out := strings.TrimRight(text, "\n") + "\n" + RenderGroupFileTask(task)
	return out, task.ID, nil
}

// backtickEach wraps each path in backticks, as a group file's files line quotes them.
func backtickEach(paths []string) []string {
	out := make([]string, len(paths))
	for index, path := range paths {
		out[index] = "`" + path + "`"
	}
	return out
}
