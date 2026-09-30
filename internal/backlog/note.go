package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// blockerLead opens a blocker note's first line, which marks where the note starts.
const blockerLead = "> **Blocked** "

// blockerTime is how a blocker note writes when its group stopped, to the minute.
const blockerTime = "2006-01-02 15:04"

// BlockerNote is a stopped group's note: when and where it stopped, why, what it needs, and its saved work.
type BlockerNote struct {
	At    time.Time
	Run   string
	State string
	Items []string
	Needs string
	Saved string
}

// Render writes the note as the blockquote that sits under the group heading.
func (n BlockerNote) Render() string {
	lines := []string{fmt.Sprintf("%s%s, run %s, at %s.", blockerLead, n.At.Format(blockerTime), n.Run, n.State)}
	for _, item := range n.Items {
		lines = append(lines, "> - "+oneLine(item))
	}
	if n.Needs != "" {
		lines = append(lines, "> - Needs: "+oneLine(n.Needs))
	}
	if n.Saved != "" {
		lines = append(lines, "> - Saved: "+oneLine(n.Saved))
	}
	return strings.Join(lines, "\n") + "\n"
}

// oneLine folds text onto one line, so a note item never breaks out of its blockquote.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// AddNote writes the note under the group's heading and its yaml block, replacing any note already there,
// and sets every task of the group not DONE to BLOCKED.
func AddNote(text, groupID string, note BlockerNote) (string, error) {
	text, _ = RemoveNote(text, groupID)
	group, ok := Parse(text).Group(groupID)
	if !ok {
		return "", fmt.Errorf("group %s not found", groupID)
	}
	lines := strings.SplitAfter(text, "\n")
	at := blockerLine(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), group.Heading)
	rendered := "\n" + note.Render() + "\n"
	lines = append(lines[:at], append([]string{rendered}, lines[at:]...)...)
	out := strings.Join(lines, "")
	var err error
	for _, task := range group.Tasks {
		if task.Status == "DONE" {
			continue
		}
		if out, err = SetStatus(out, task.ID, "BLOCKED"); err != nil {
			return "", err
		}
	}
	return out, nil
}

// RemoveNote deletes the group's blocker note and the blank lines around it, reporting whether it held one.
func RemoveNote(text, groupID string) (string, bool) {
	group, ok := Parse(text).Group(groupID)
	if !ok {
		return text, false
	}
	lines := strings.SplitAfter(text, "\n")
	at := blockerLine(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), group.Heading)
	start := at
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || !strings.HasPrefix(lines[start], blockerLead) {
		return text, false
	}
	end := start
	for end < len(lines) && strings.HasPrefix(lines[end], ">") {
		end++
	}
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}
	return strings.Join(append(lines[:at], lines[end:]...), ""), true
}

// GroupText returns the group's section: its heading and every line up to the next group or epic heading.
func GroupText(text, groupID string) (string, bool) {
	group, ok := Parse(text).Group(groupID)
	if !ok {
		return "", false
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	end := group.Heading + 1
	for end < len(lines) && !groupHeading.MatchString(lines[end]) && !epicHeading.MatchString(lines[end]) {
		end++
	}
	return strings.TrimSpace(strings.Join(lines[group.Heading:end], "\n")), true
}

// blockerLine is the line index a group's note goes at: just past its heading and the yaml block under it.
func blockerLine(lines []string, heading int) int {
	if _, _, end, err := readBlock(lines, heading+1); err == nil && end >= 0 {
		return end + 1
	}
	return heading + 1
}

// AddGroupFileNote writes the note under a docs/backlog group file's heading and yaml block,
// replacing any note already there, and sets the group's own heading status to BLOCKED.
func AddGroupFileNote(text string, note BlockerNote) (string, error) {
	text, _ = RemoveGroupFileNote(text)
	if ParseGroupFile(text).ID == "" {
		return "", fmt.Errorf("no group heading found")
	}
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	at := groupFileBlockerLine(rawLines)
	lines := strings.SplitAfter(text, "\n")
	rendered := "\n" + note.Render() + "\n"
	lines = append(lines[:at], append([]string{rendered}, lines[at:]...)...)
	return setGroupFileHeadingStatus(strings.Join(lines, ""), "BLOCKED")
}

// RemoveGroupFileNote deletes a docs/backlog group file's blocker note and the blank lines around
// it, reporting whether it held one.
func RemoveGroupFileNote(text string) (string, bool) {
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	at := groupFileBlockerLine(rawLines)
	lines := strings.SplitAfter(text, "\n")
	start := at
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || !strings.HasPrefix(lines[start], blockerLead) {
		return text, false
	}
	end := start
	for end < len(lines) && strings.HasPrefix(lines[end], ">") {
		end++
	}
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}
	return strings.Join(append(lines[:at], lines[end:]...), ""), true
}

// groupFileBlockerLine is the line index a group file's note goes at: just past its heading and
// the yaml block under it.
func groupFileBlockerLine(lines []string) int {
	for index, line := range lines {
		if groupFileHeading.MatchString(line) {
			if _, _, end, err := readBlock(lines, index+1); err == nil && end >= 0 {
				return end + 1
			}
			return index + 1
		}
	}
	return 0
}

// setGroupFileHeadingStatus rewrites a group file's own heading status token, leaving the rest of
// the file untouched.
func setGroupFileHeadingStatus(text, status string) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	for index, line := range lines {
		bare := strings.TrimRight(line, "\r\n")
		if groupFileHeading.MatchString(bare) {
			lines[index] = statusToken.ReplaceAllString(bare, "["+status+"]") + line[len(bare):]
			return strings.Join(lines, ""), nil
		}
	}
	return "", fmt.Errorf("no group heading found")
}

// FindGroupFile returns the path and text of the docs/backlog group file whose heading names
// groupID, if root holds one.
func FindGroupFile(root, groupID string) (path, text string, found bool, err error) {
	entries, err := os.ReadDir(filepath.Join(root, GroupFilesDir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		candidate := filepath.Join(root, GroupFilesDir, name)
		data, err := os.ReadFile(candidate)
		if err != nil {
			return "", "", false, err
		}
		if ParseGroupFile(string(data)).ID == groupID {
			return candidate, string(data), true, nil
		}
	}
	return "", "", false, nil
}
