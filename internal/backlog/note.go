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
