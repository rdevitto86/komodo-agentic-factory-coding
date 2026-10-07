package check

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CoverageTolerance is how far changed-line coverage may fall below its calibrated bar before it fails.
const CoverageTolerance = 0.01

// AddedLine is one line a diff added, in the file it lands in after the change.
type AddedLine struct {
	File string
	Line int
	Text string
}

// ParseAddedLines reads a unified diff, returning every added line.
// Each hunk's own line counts, not a line's prefix, mark body.
func ParseAddedLines(diff string) []AddedLine {
	var added []AddedLine
	var file string
	var next, oldLeft, newLeft int
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case oldLeft > 0 || newLeft > 0:
			switch {
			case strings.HasPrefix(line, "\\"):
				// A "\ No newline at end of file" marker names no line in either file.
			case strings.HasPrefix(line, "+"):
				added = append(added, AddedLine{File: file, Line: next, Text: strings.TrimPrefix(line, "+")})
				next++
				newLeft--
			case strings.HasPrefix(line, "-"):
				oldLeft--
			default:
				next++
				oldLeft--
				newLeft--
			}
		case strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@ "):
			next, oldLeft, newLeft = hunkHeader(line)
		}
	}
	return added
}

// hunkHeader reads a "@@ -a,b +c,d @@" line into the new file's starting line and both line counts.
func hunkHeader(header string) (newStart, oldCount, newCount int) {
	for _, field := range strings.Fields(header) {
		switch {
		case strings.HasPrefix(field, "-"):
			_, oldCount = hunkRange(field)
		case strings.HasPrefix(field, "+"):
			newStart, newCount = hunkRange(field)
		}
	}
	return newStart, oldCount, newCount
}

// hunkRange parses one "-a,b" or "+c,d" hunk field into its start line and count, the count
// defaulting to 1 when the header omits it.
func hunkRange(field string) (start, count int) {
	numeric, countText, hasComma := strings.Cut(field[1:], ",")
	start, _ = strconv.Atoi(numeric)
	count = 1
	if hasComma {
		count, _ = strconv.Atoi(countText)
	}
	return start, count
}

// block is one cover profile range, inclusive of both its start and end line.
type block struct {
	start, end, count int
}

// Profile is a parsed cover profile's blocks, keyed by file.
type Profile map[string][]block

// ModulePath reads the module path from the go.mod at the root of worktree.
func ModulePath(worktree string) (string, error) {
	data, err := os.ReadFile(filepath.Join(worktree, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod names no module", worktree)
}

// ParseCoverProfile parses go test's -coverprofile output into a Profile, keying each file by its
// path relative to module so it matches a diff's paths.
func ParseCoverProfile(text, module string) (Profile, error) {
	profile := Profile{}
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		file, b, err := parseCoverLine(line)
		if err != nil {
			return nil, err
		}
		file = strings.TrimPrefix(file, module+"/")
		profile[file] = append(profile[file], b)
	}
	return profile, nil
}

// parseCoverLine parses one "file:startLine.col,endLine.col numStmt count" cover profile line.
func parseCoverLine(line string) (string, block, error) {
	file, rest, ok := strings.Cut(line, ":")
	if !ok {
		return "", block{}, fmt.Errorf("malformed cover line: %q", line)
	}
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return "", block{}, fmt.Errorf("malformed cover line: %q", line)
	}
	startPos, endPos, ok := strings.Cut(fields[0], ",")
	if !ok {
		return "", block{}, fmt.Errorf("malformed cover line: %q", line)
	}
	start, _, ok := strings.Cut(startPos, ".")
	if !ok {
		return "", block{}, fmt.Errorf("malformed cover line: %q", line)
	}
	end, _, ok := strings.Cut(endPos, ".")
	if !ok {
		return "", block{}, fmt.Errorf("malformed cover line: %q", line)
	}
	startLine, err := strconv.Atoi(start)
	if err != nil {
		return "", block{}, err
	}
	endLine, err := strconv.Atoi(end)
	if err != nil {
		return "", block{}, err
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", block{}, err
	}
	return file, block{start: startLine, end: endLine, count: count}, nil
}

// Covered reports whether line in file falls inside a counted block, and whether the profile names that line at all.
func (p Profile) Covered(file string, line int) (covered, known bool) {
	for _, b := range p[file] {
		if line >= b.start && line <= b.end {
			return b.count > 0, true
		}
	}
	return false, false
}

// ChangedLineCoverage is the fraction of added .go lines the profile covers, among the lines it knows about,
// and whether it knows any; a diff with none it recognises reports full coverage and false.
func ChangedLineCoverage(diff string, profile Profile) (float64, bool) {
	var known, covered int
	for _, line := range ParseAddedLines(diff) {
		if !strings.HasSuffix(line.File, ".go") {
			continue
		}
		isCovered, isKnown := profile.Covered(line.File, line.Line)
		if !isKnown {
			continue
		}
		known++
		if isCovered {
			covered++
		}
	}
	if known == 0 {
		return 1, false
	}
	return float64(covered) / float64(known), true
}

// bar is what a coverage bar file persists: the calibrated floor changed-line coverage must hold.
type bar struct {
	Value float64 `json:"value"`
}

// Evaluate compares percent against the bar stored at path, calibrating it to percent only when no bar
// exists, and returns a problem when coverage falls more than CoverageTolerance below it.
func Evaluate(path string, percent float64) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, saveBar(path, percent)
	}
	if err != nil {
		return nil, err
	}
	var stored bar
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	if percent < stored.Value-CoverageTolerance {
		return []string{fmt.Sprintf(
			"coverage: changed-line coverage %.1f%% fell below its calibrated bar of %.1f%%",
			percent*100, stored.Value*100,
		)}, nil
	}
	return nil, nil
}

// saveBar writes a coverage bar's value to path.
func saveBar(path string, value float64) error {
	data, err := json.Marshal(bar{Value: value})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
