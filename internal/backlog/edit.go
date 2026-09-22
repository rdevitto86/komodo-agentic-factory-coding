package backlog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var statusToken = regexp.MustCompile(`\[[A-Z_]+\]\s*$`)

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

// AppendTask adds a task at the end of a group and returns the new text and the new id.
func AppendTask(text, groupID, title string, fields Fields, priority, status string) (string, string, error) {
	if !contains(Priorities, priority) {
		return "", "", fmt.Errorf("unknown priority %q", priority)
	}
	if !contains(Statuses, status) {
		return "", "", fmt.Errorf("unknown status %q", status)
	}
	parsed := Parse(text)
	group, ok := parsed.Group(groupID)
	if !ok {
		return "", "", fmt.Errorf("group %s not found", groupID)
	}
	taskID := NextTaskID(group)
	lines := strings.SplitAfter(text, "\n")
	position := len(lines)
	for _, candidate := range parsed.Groups {
		if candidate.Heading > group.Heading {
			position = candidate.Heading
			break
		}
	}
	for position > group.Heading+1 && strings.TrimSpace(lines[position-1]) == "" {
		position--
	}
	block := RenderTask(taskID, title, priority, status, fields)
	if position > 0 && strings.TrimSpace(lines[position-1]) != "" {
		block = "\n" + block
	}
	if position < len(lines) {
		block += "\n"
	}
	out := append([]string{}, lines[:position]...)
	out = append(out, block)
	out = append(out, lines[position:]...)
	return strings.Join(out, ""), taskID, nil
}
