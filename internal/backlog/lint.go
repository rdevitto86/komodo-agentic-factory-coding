package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"komodo/internal/changelog"
)

// MaxGroupTasks is the most buildable tasks one group holds before lint suggests a split.
const MaxGroupTasks = 12

var slugDrop = regexp.MustCompile(`[^a-z0-9_ -]+`)

// Slug is the anchor form of a heading, matching GitHub: lower case, punctuation
// dropped except - and _, each space becomes a dash.
func Slug(text string) string {
	return strings.ReplaceAll(slugDrop.ReplaceAllString(strings.ToLower(text), ""), " ", "-")
}

// Lint returns every problem that would stop the line running this backlog deterministically.
func Lint(parsed Backlog) []string {
	problems := append([]string(nil), parsed.Problems...)
	seen := map[string]int{}
	ids := map[string]bool{}
	groupIDs := map[string]bool{}
	var mismatchEpics []string
	mismatchGroups := map[string][]string{}
	mismatchVersions := map[string]string{}
	for _, task := range parsed.Tasks() {
		ids[task.ID] = true
	}
	for _, group := range parsed.Groups {
		groupIDs[group.ID] = true
	}
	for _, group := range parsed.Groups {
		if !contains(Modes, group.Mode()) {
			problems = append(problems, fmt.Sprintf("%s: mode must be one of %s", group.ID, strings.Join(Modes, "|")))
		}
		if !contains(Types, group.Type()) {
			problems = append(problems, fmt.Sprintf("%s: type must be one of %s", group.ID, strings.Join(Types, "|")))
		}
		switch version := group.Version(); {
		case version == "":
			problems = append(problems, fmt.Sprintf("%s: no version; a group declares the version it ships as `version: x.y.z`", group.ID))
		case !versionRe.MatchString(version):
			problems = append(problems, fmt.Sprintf("%s: version %q is not x.y.z", group.ID, version))
		case !versionPhaseRe.MatchString(version):
			problems = append(problems, fmt.Sprintf(
				"%s: version %q must be x.y.z, or x.y.z-alpha.n, -beta.n or -rc.n, the four phases alpha, beta, rc, stable",
				group.ID, version))
		default:
			if group.EpicID != "" {
				if epic, ok := parsed.Epic(group.EpicID); ok {
					epicVersion := epic.Version()
					if epicVersion == "" {
						problems = append(problems, fmt.Sprintf("%s: epic %s has no version; an epic names the version it ships as Ships as `x.y.z`", group.ID, group.EpicID))
					} else if epicVersion != version {
						if _, seen := mismatchGroups[group.EpicID]; !seen {
							mismatchEpics = append(mismatchEpics, group.EpicID)
						}
						mismatchGroups[group.EpicID] = append(mismatchGroups[group.EpicID], group.ID)
						mismatchVersions[group.EpicID] = epicVersion
					}
				}
			}
		}
		if line, dup := seen[group.ID]; dup {
			problems = append(problems, fmt.Sprintf("%s: duplicate group id (lines %d and %d)", group.ID, line+1, group.Heading+1))
		}
		seen[group.ID] = group.Heading
		// Filed findings wait in REFINEMENT and no builder works them, so only the rest count toward the cap.
		if built := buildable(group); built > MaxGroupTasks {
			problems = append(problems, fmt.Sprintf("%s: %d tasks exceeds limit of %d (suggest a split per REQ-8)", group.ID, built, MaxGroupTasks))
		}
		var depBranches, depTitleBranches []string
		for _, dep := range group.DependsOn() {
			if !groupIDs[dep] {
				problems = append(problems, fmt.Sprintf("%s: depends_on names unknown group %s", group.ID, dep))
				continue
			}
			if depGroup, ok := parsed.Group(dep); ok {
				depBranches = append(depBranches, depGroup.Branch())
				depTitleBranches = append(depTitleBranches, depGroup.TitleBranch())
				if own, later := group.Version(), depGroup.Version(); own != "" && later != "" && changelog.Compare(own, later) < 0 {
					problems = append(problems, fmt.Sprintf(
						"%s: depends_on names %s at a later version (%s depends on %s)", group.ID, dep, own, later))
				}
			}
		}
		epicBranch := group.EpicBranch()
		switch base := group.Base(); {
		case base == "" || base == epicBranch || contains(depBranches, base):
		case contains(depTitleBranches, base):
			problems = append(problems, fmt.Sprintf(
				"%s: base %q names a dependency's title-only branch, which a new cut never creates; use its id form instead", group.ID, base))
		default:
			problems = append(problems, fmt.Sprintf(
				"%s: base %q is neither its epic branch %q nor the branch of a group named in depends_on", group.ID, base, epicBranch))
		}
	}
	for _, epicID := range mismatchEpics {
		problems = append(problems, fmt.Sprintf("%s: version %q disagrees with groups %s; split the epic per version",
			epicID, mismatchVersions[epicID], strings.Join(mismatchGroups[epicID], ", ")))
	}
	for _, task := range parsed.Tasks() {
		where := fmt.Sprintf("%s (line %d)", task.ID, task.Heading+1)
		if _, dup := seen[task.ID]; dup {
			problems = append(problems, where+": duplicate task id")
		}
		seen[task.ID] = task.Heading
		if !contains(Statuses, task.Status) {
			problems = append(problems, fmt.Sprintf("%s: status must be one of %s", where, strings.Join(Statuses, "|")))
		}
		if !contains(Priorities, task.Priority) {
			problems = append(problems, fmt.Sprintf("%s: priority must be one of %s", where, strings.Join(Priorities, "|")))
		}
		if !contains(Owners, task.Owner()) {
			problems = append(problems, where+": owner must be agent or human")
		}
		if !contains(Types, task.Type()) {
			problems = append(problems, fmt.Sprintf("%s: type must be one of %s", where, strings.Join(Types, "|")))
		}
		if timeout := task.Timeout(); timeout != "" {
			if parsed, err := time.ParseDuration(timeout); err != nil || parsed <= 0 {
				problems = append(problems, fmt.Sprintf("%s: timeout %q is not a positive duration such as 15m", where, timeout))
			}
		}
		if tier := task.Tier(); tier != "" && !contains(Tiers, tier) {
			problems = append(problems, fmt.Sprintf("%s: tier must be one of %s", where, strings.Join(Tiers, "|")))
		}
		if task.Owner() == "agent" && task.Tier() == "light" {
			problems = append(problems, fmt.Sprintf("%s: agent task cannot have tier light (REQ-30)", where))
		}
		for _, dep := range task.DependsOn() {
			if !ids[dep] {
				problems = append(problems, fmt.Sprintf("%s: depends_on names unknown task %s", where, dep))
			}
		}
		if task.Owner() != "agent" || !task.Ready() {
			continue
		}
		if task.BlockStart < 0 {
			problems = append(problems, where+": agent task has no yaml block")
			continue
		}
		if len(task.Files()) == 0 {
			problems = append(problems, where+": agent task declares no files")
		}
		if len(task.DoneWhen()) == 0 {
			problems = append(problems, where+": agent task declares no done_when commands")
		}
		for _, command := range task.DoneWhen() {
			if !commandHint.MatchString(strings.TrimSpace(command)) {
				problems = append(problems, fmt.Sprintf("%s: done_when entry does not look like a command: %q", where, command))
			}
		}
	}
	return problems
}

// Advice that never fails lint: each group whose longest open dependency chain covers most of its open tasks.
func Notes(parsed Backlog) []string {
	var notes []string
	for _, group := range parsed.Groups {
		open := map[string]Task{}
		for _, task := range group.Tasks {
			if task.Status != "DONE" {
				open[task.ID] = task
			}
		}
		if len(open) < 3 {
			continue
		}
		depth := map[string]int{}
		var chain func(string) int
		chain = func(id string) int {
			if known, ok := depth[id]; ok {
				return known
			}
			depth[id] = 1
			longest := 0
			for _, dep := range open[id].DependsOn() {
				if _, ok := open[dep]; ok {
					longest = max(longest, chain(dep))
				}
			}
			depth[id] = longest + 1
			return depth[id]
		}
		longest := 0
		for id := range open {
			longest = max(longest, chain(id))
		}
		if longest*2 > len(open) {
			notes = append(notes, fmt.Sprintf("%s: %d of %d tasks are one chain; split the shared file or drop a dependency to build in parallel",
				group.ID, longest, len(open)))
		}
	}
	for _, task := range parsed.Tasks() {
		if task.Owner() != "agent" || !task.Ready() {
			continue
		}
		if untestedCaller(task.Files(), task.DoneWhen()) {
			notes = append(notes, untestedCallerNote(task.ID))
		}
	}
	return notes
}

// Group-file advice matching Notes: an untested-caller note for each open agent task in one file.
func NotesGroupFile(file GroupFile) []string {
	var notes []string
	for _, task := range file.Tasks {
		if task.Done {
			continue
		}
		status := task.Status
		if status == "" {
			status = file.Status
		}
		if status != "READY" && status != "IN_PROGRESS" {
			continue
		}
		owner := task.Owner
		if owner == "" {
			owner = "agent"
		}
		if owner != "agent" {
			continue
		}
		if untestedCaller(task.Files, task.Checks) {
			notes = append(notes, untestedCallerNote(task.ID))
		}
	}
	return notes
}

// untestedCallerNote is the note text for a task whose done_when tests only its own package.
func untestedCallerNote(id string) string {
	return fmt.Sprintf("%s: done_when only runs go test of its own package(s); files name no caller such as cmd/komodo, the conductor, or a hook", id)
}

// untestedCaller reports whether every done_when command is a go test of exactly the packages the
// task's own files live in, so nothing in files could call the changed code in.
func untestedCaller(files, doneWhen []string) bool {
	if len(files) == 0 || len(doneWhen) == 0 {
		return false
	}
	var packages []string
	for _, command := range doneWhen {
		pkgs, ok := goTestPackages(command)
		if !ok {
			return false
		}
		packages = append(packages, pkgs...)
	}
	for _, file := range files {
		clean := strings.ReplaceAll(file, "\\", "/")
		if strings.HasPrefix(clean, "cmd/") || strings.Contains(clean, "/cmd/") {
			return false
		}
		dir := clean
		if filepath.Ext(clean) != "" {
			dir = filepath.Dir(clean)
		}
		if !coveredByAPackage(dir, packages) {
			return false
		}
	}
	return true
}

// goTestPackages splits a `go test <pkg>...` command into the package paths it names, ok false for
// any other shape: a flag, another subcommand, or a chained command.
func goTestPackages(command string) (packages []string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "test" {
		return nil, false
	}
	for _, field := range fields[2:] {
		if !strings.HasPrefix(field, "./") {
			return nil, false
		}
		packages = append(packages, strings.TrimSuffix(strings.TrimPrefix(field, "./"), "/..."))
	}
	return packages, len(packages) > 0
}

// coveredByAPackage reports whether dir is a package go test already ran: itself or one of its subdirectories.
func coveredByAPackage(dir string, packages []string) bool {
	for _, pkg := range packages {
		if dir == pkg || strings.HasPrefix(dir, pkg+"/") {
			return true
		}
	}
	return false
}

// LintContext reports every context anchor whose file exists under root but holds no matching
// heading, so a mistyped anchor fails at lint instead of sending a builder the whole file.
func LintContext(root string, parsed Backlog) []string {
	var problems []string
	for _, task := range parsed.Tasks() {
		if !task.Open() {
			continue
		}
		for _, ref := range task.Context() {
			path, anchor, found := strings.Cut(ref, "#")
			if !found || anchor == "" || strings.ContainsAny(ref, " \t") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				continue
			}
			if !hasHeading(string(data), anchor) {
				problems = append(problems, fmt.Sprintf("%s (line %d): context %s names no heading in %s", task.ID, task.Heading+1, ref, path))
			}
		}
	}
	return problems
}

// hasHeading reports whether a markdown text holds a heading whose slug matches the anchor.
func hasHeading(text, anchor string) bool {
	want := Slug(anchor)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") && Slug(strings.TrimSpace(strings.TrimLeft(line, "#"))) == want {
			return true
		}
	}
	return false
}

var groupFileContextLine = regexp.MustCompile(`^\s{2}-\s+context:\s*(.*)$`)

// LintGroupFile checks one docs/backlog group file as Lint checks a legacy backlog's group: version,
// an open task's files, a context anchor, and depends_on naming a known group or task at no later version.
func LintGroupFile(root string, file GroupFile, text string, groupIDs, taskIDs map[string]bool, groupVersions map[string]string) []string {
	var problems []string
	switch version := file.Version; {
	case version == "":
		problems = append(problems, fmt.Sprintf("%s: no version; a group declares the version it ships as `version: x.y.z`", file.ID))
	case !versionRe.MatchString(version):
		problems = append(problems, fmt.Sprintf("%s: version %q is not x.y.z", file.ID, version))
	case !versionPhaseRe.MatchString(version):
		problems = append(problems, fmt.Sprintf(
			"%s: version %q must be x.y.z, or x.y.z-alpha.n, -beta.n or -rc.n, the four phases alpha, beta, rc, stable",
			file.ID, version))
	}
	switch groupType := file.Type; {
	case groupType == "":
		problems = append(problems, fmt.Sprintf("%s: no type; a group declares its branch and commit type `type: feat`", file.ID))
	case !contains(Types, groupType):
		problems = append(problems, fmt.Sprintf("%s: type must be one of %s", file.ID, strings.Join(Types, "|")))
	}
	for _, dep := range file.DependsOn {
		if !groupIDs[dep] && !taskIDs[dep] {
			problems = append(problems, fmt.Sprintf("%s: depends_on names unknown group or task %s", file.ID, dep))
			continue
		}
		if own, later := file.Version, groupVersions[dep]; own != "" && later != "" && changelog.Compare(own, later) < 0 {
			problems = append(problems, fmt.Sprintf(
				"%s: depends_on names %s at a later version (%s depends on %s)", file.ID, dep, own, later))
		}
	}
	for _, task := range file.Tasks {
		status := task.Status
		if status == "" {
			status = file.Status
		}
		owner := task.Owner
		if owner == "" {
			owner = "agent"
		}
		ready := status == "READY" || status == "IN_PROGRESS"
		// A person's own task, or one still in REFINEMENT, needs neither files nor done_when yet.
		if task.Done || owner != "agent" || !ready {
			continue
		}
		if len(task.Files) == 0 {
			problems = append(problems, fmt.Sprintf("%s: open task declares no files", task.ID))
		}
		if len(task.Checks) == 0 {
			problems = append(problems, fmt.Sprintf("%s: agent task declares no done_when commands", task.ID))
		}
	}
	for _, line := range strings.Split(text, "\n") {
		match := groupFileContextLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, ref := range splitGroupFileList(match[1]) {
			path, anchor, found := strings.Cut(ref, "#")
			if !found || anchor == "" || strings.ContainsAny(ref, " \t") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				continue
			}
			if !hasHeading(string(data), anchor) {
				problems = append(problems, fmt.Sprintf("%s: context %s names no heading in %s", file.ID, ref, path))
			}
		}
	}
	return problems
}

// LintGroupFileEpics reports one problem per epic whose docs/backlog group files disagree on the
// version it ships, naming every group that differs from the first one seen.
func LintGroupFileEpics(files []GroupFile) []string {
	type mismatch struct {
		version string
		groups  []string
	}
	epics := map[string]*mismatch{}
	var order []string
	for _, file := range files {
		if file.EpicID == "" || file.Version == "" {
			continue
		}
		entry, ok := epics[file.EpicID]
		if !ok {
			epics[file.EpicID] = &mismatch{version: file.Version}
			order = append(order, file.EpicID)
			continue
		}
		if file.Version != entry.version {
			entry.groups = append(entry.groups, file.ID)
		}
	}
	var problems []string
	for _, epicID := range order {
		entry := epics[epicID]
		if len(entry.groups) == 0 {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s: version %q disagrees with groups %s; split the epic per version",
			epicID, entry.version, strings.Join(entry.groups, ", ")))
	}
	return problems
}

// LintGroupFileDuplicates reports a problem for every group or task id more than one docs/backlog
// file declares, so findGroupFile never appends to whichever file happens to sort first.
func LintGroupFileDuplicates(files []GroupFile) []string {
	var problems []string
	groupSeen := map[string]bool{}
	for _, file := range files {
		if file.ID == "" {
			continue
		}
		if groupSeen[file.ID] {
			problems = append(problems, fmt.Sprintf("%s: duplicate group id, declared by more than one file", file.ID))
		}
		groupSeen[file.ID] = true
	}
	taskSeen := map[string]bool{}
	for _, file := range files {
		for _, task := range file.Tasks {
			if taskSeen[task.ID] {
				problems = append(problems, fmt.Sprintf("%s: duplicate task id, declared by more than one file", task.ID))
			}
			taskSeen[task.ID] = true
		}
	}
	return problems
}

// LintVersions reports every open group whose version, or a prerelease of it, sorts at or behind
// tags' newest version; nil tags, from no repo or none cut, skip the check.
func LintVersions(parsed Backlog, tags []string) []string {
	newest := newestTag(tags)
	if newest == "" {
		return nil
	}
	var problems []string
	for _, group := range parsed.Groups {
		version := group.Version()
		if version == "" || !versionRe.MatchString(version) || !groupOpen(group) {
			continue
		}
		if changelog.Compare(version, newest) <= 0 {
			problems = append(problems, fmt.Sprintf(
				"%s: version %q is at or behind the newest tag %q", group.ID, version, newest))
		}
	}
	return problems
}

// newestTag is the highest version among tags, a leading v stripped, or empty with none.
func newestTag(tags []string) string {
	newest := ""
	for _, tag := range tags {
		version := strings.TrimPrefix(tag, "v")
		if !versionRe.MatchString(version) {
			continue
		}
		if newest == "" || changelog.Compare(version, newest) > 0 {
			newest = version
		}
	}
	return newest
}

// groupOpen reports whether a group still has work: no tasks yet, or any task not DONE or BLOCKED.
func groupOpen(group Group) bool {
	if len(group.Tasks) == 0 {
		return true
	}
	for _, task := range group.Tasks {
		if task.Open() {
			return true
		}
	}
	return false
}

// BuildableGroupFile counts a group file's tasks a builder session works: every one not waiting in
// REFINEMENT, its own status or the group's own when the task carries none.
func BuildableGroupFile(file GroupFile) int {
	count := 0
	for _, task := range file.Tasks {
		status := task.Status
		if status == "" {
			status = file.Status
		}
		if status != "REFINEMENT" {
			count++
		}
	}
	return count
}

// NextGroupID is the next free group id for the given epic id, skipping every id already taken
// so a collision on an unmerged branch is never proposed again.
func NextGroupID(epicID string, taken map[string]bool) (string, error) {
	num := strings.TrimPrefix(epicID, "EPIC-")
	if num == "" || num == epicID {
		return "", fmt.Errorf("not an epic id: %q", epicID)
	}
	prefix := "TG-" + num + "."
	highest := 0
	for id := range taken {
		if suffix, ok := strings.CutPrefix(id, prefix); ok {
			if n, err := strconv.Atoi(suffix); err == nil && n > highest {
				highest = n
			}
		}
	}
	return fmt.Sprintf("%s%d", prefix, highest+1), nil
}

// buildable counts the group's tasks a builder session works: every one not waiting in REFINEMENT.
func buildable(group Group) int {
	count := 0
	for _, task := range group.Tasks {
		if task.Status != "REFINEMENT" {
			count++
		}
	}
	return count
}
