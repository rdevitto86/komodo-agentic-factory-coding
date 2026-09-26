// Package changelog reads CHANGELOG.md with every group's fragment folded in, and orders versions.
package changelog

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// File is the changelog every reader sees, fragments folded in.
const File = "CHANGELOG.md"

// Dir holds one fragment per shipped group, as Dir/<version>/<group>.md, until a fold writes it into File.
const Dir = "changelog.d"

// FragmentPath is where a group's line for a version is written.
func FragmentPath(root, version, group string) string {
	return filepath.Join(root, Dir, version, group+".md")
}

// WriteFragment writes a group's one changelog line for a version, replacing any it wrote before.
func WriteFragment(root, version, group, line string) error {
	path := FragmentPath(root, version, group)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(line)+"\n"), 0o644)
}

// Fragments maps each version under root's Dir to its fragments' lines, newest group first.
func Fragments(root string) (map[string][]string, error) {
	versions, err := os.ReadDir(filepath.Join(root, Dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, Dir, version.Name(), "*.md"))
		if err != nil {
			return nil, err
		}
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if line = strings.TrimRight(line, "\r"); line != "" {
					out[version.Name()] = append(out[version.Name()], line)
				}
			}
		}
	}
	return out, nil
}

// Read returns root's CHANGELOG.md with its fragments folded in, or the empty string when it has neither.
func Read(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, File))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	fragments, err := Fragments(root)
	if err != nil {
		return "", err
	}
	return Fold(string(data), fragments), nil
}

// FoldFiles writes the fragments into root's CHANGELOG.md, dating each new heading, and deletes them.
func FoldFiles(root, date string) error {
	data, err := os.ReadFile(filepath.Join(root, File))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	fragments, err := Fragments(root)
	if err != nil || len(fragments) == 0 {
		return err
	}
	text := foldDated(string(data), fragments, date)
	if err := os.WriteFile(filepath.Join(root, File), []byte(text), 0o644); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(root, Dir))
}

// versionHeading matches a version heading with or without brackets, a leading v, or a date.
var versionHeading = regexp.MustCompile(`(?m)^##\s+\[?v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\]?.*$`)

// anyHeading is any second-level heading: a version, or a titled history section.
var anyHeading = regexp.MustCompile(`(?m)^## `)

// Fold writes each version's fragment lines into text under its heading, adding a heading where none exists.
func Fold(text string, fragments map[string][]string) string {
	return foldDated(text, fragments, "")
}

// foldDated folds fragments into text; a heading it adds carries the date when one is given.
func foldDated(text string, fragments map[string][]string, date string) string {
	versions := make([]string, 0, len(fragments))
	for version := range fragments {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return Compare(versions[i], versions[j]) < 0 })
	for _, version := range versions {
		lines := fragments[version]
		start, found := sectionStart(text, version)
		if !found {
			heading := "## " + version
			if date != "" {
				heading += " — " + date
			}
			text, start = insertHeading(text, version, heading)
		}
		text = intoSection(text, start, lines)
	}
	return text
}

// sectionStart is the offset just past version's heading line.
func sectionStart(text, version string) (int, bool) {
	for _, match := range versionHeading.FindAllStringSubmatchIndex(text, -1) {
		if text[match[2]:match[3]] == version {
			end := match[1]
			if end < len(text) {
				end++
			}
			return end, true
		}
	}
	return 0, false
}

// insertHeading adds heading above the first version heading older than version; with none older, it goes
// below the last version's section, or above the first heading when no version has one yet.
func insertHeading(text, version, heading string) (string, int) {
	at := -1
	versions := versionHeading.FindAllStringSubmatchIndex(text, -1)
	for _, match := range versions {
		if Compare(text[match[2]:match[3]], version) < 0 {
			at = match[0]
			break
		}
	}
	if at < 0 && len(versions) > 0 {
		last := versions[len(versions)-1][1]
		at = len(text)
		if next := anyHeading.FindStringIndex(text[last:]); next != nil {
			at = last + next[0]
		}
	}
	if at < 0 {
		if first := anyHeading.FindStringIndex(text); first != nil {
			at = first[0]
		}
	}
	if at < 0 || at == len(text) {
		if text == "" {
			text = "# Changelog\n"
		}
		text = strings.TrimRight(text, "\n") + "\n\n"
		return text + heading + "\n", len(text) + len(heading) + 1
	}
	return text[:at] + heading + "\n\n" + text[at:], at + len(heading) + 1
}

// groupBullet matches a group's changelog line and captures its id.
var groupBullet = regexp.MustCompile(`(?m)^- \*\*(TG-[^*]+)\*\*.*$`)

// intoSection writes lines at the top of the section starting at start: a group's line replaces the one it
// wrote before, and a line the section already holds is skipped.
func intoSection(text string, start int, lines []string) string {
	end := len(text)
	if next := anyHeading.FindStringIndex(text[start:]); next != nil {
		end = start + next[0]
	}
	section := text[start:end]
	var fresh []string
	for _, line := range lines {
		if strings.Contains("\n"+section+"\n", "\n"+line+"\n") {
			continue
		}
		if replaced, ok := replaceBullet(section, line); ok {
			section = replaced
			continue
		}
		fresh = append(fresh, line)
	}
	if len(fresh) == 0 {
		return text[:start] + section + text[end:]
	}
	body := strings.TrimLeft(section, "\n")
	joined := strings.Join(fresh, "\n") + "\n"
	if strings.TrimSpace(body) == "" {
		if end == len(text) {
			return text[:start] + "\n" + joined
		}
		return text[:start] + "\n" + joined + "\n" + text[end:]
	}
	return text[:start] + "\n" + joined + body + text[end:]
}

// replaceBullet swaps the section's line for line's group, reporting whether it held one.
func replaceBullet(section, line string) (string, bool) {
	own := groupBullet.FindStringSubmatch(line)
	if own == nil {
		return section, false
	}
	for _, bullet := range groupBullet.FindAllStringSubmatchIndex(section, -1) {
		if section[bullet[2]:bullet[3]] == own[1] {
			return section[:bullet[0]] + line + section[bullet[1]:], true
		}
	}
	return section, false
}

// Latest is the highest version text names, or the empty string when it names none.
func Latest(text string) string {
	latest := ""
	for _, match := range versionHeading.FindAllStringSubmatch(text, -1) {
		if latest == "" || Compare(match[1], latest) > 0 {
			latest = match[1]
		}
	}
	return latest
}

// Compare orders two semantic versions, returning -1, 0, or 1; a prerelease sorts before its release.
func Compare(left, right string) int {
	leftCore, leftPre, _ := strings.Cut(left, "-")
	rightCore, rightPre, _ := strings.Cut(right, "-")
	a, b := parts(leftCore), parts(rightCore)
	for index := 0; index < 3; index++ {
		if a[index] != b[index] {
			if a[index] < b[index] {
				return -1
			}
			return 1
		}
	}
	switch {
	case leftPre == rightPre:
		return 0
	case leftPre == "":
		return 1
	case rightPre == "":
		return -1
	}
	return comparePrerelease(leftPre, rightPre)
}

// comparePrerelease orders two prerelease strings field by field, numbers numerically.
func comparePrerelease(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < len(a) && index < len(b); index++ {
		x, xErr := strconv.Atoi(a[index])
		y, yErr := strconv.Atoi(b[index])
		switch {
		case xErr == nil && yErr == nil && x != y:
			if x < y {
				return -1
			}
			return 1
		case (xErr == nil) != (yErr == nil):
			if xErr == nil {
				return -1
			}
			return 1
		case a[index] != b[index]:
			return strings.Compare(a[index], b[index])
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// parts splits a version core into its three numbers.
func parts(version string) [3]int {
	var out [3]int
	for index, field := range strings.SplitN(version, ".", 3) {
		if index > 2 {
			break
		}
		number, err := strconv.Atoi(strings.TrimSpace(field))
		if err == nil {
			out[index] = number
		}
	}
	return out
}
