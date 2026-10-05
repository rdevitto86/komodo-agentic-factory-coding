package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// EpicFileName is the index file of an epic folder.
const EpicFileName = "EPIC.md"

// GroupFileName is the index file of a group folder.
const GroupFileName = "TG.md"

// DefaultGroupsMax is how many groups an epic holds unless its the epic index file raises groups_max.
const DefaultGroupsMax = 6

var (
	epicFileHeading = regexp.MustCompile(`^##\s+\[(EPIC-[\w.]+)\]\s+(.+?)\s*\[([A-Z_]+)\]\s*$`)
	epicDirPattern  = regexp.MustCompile(`^epic-[\w.]+$`)
	groupDirPattern = regexp.MustCompile(`^tg-[\w.]+$`)
	taskFilePattern = regexp.MustCompile(`^tsk-[\w.]+\.md$`)
	numberRun       = regexp.MustCompile(`\d+|\D+`)
)

// EpicFile is one parsed an epic index file: heading, yaml, and the goal paragraph under them.
type EpicFile struct {
	ID        string
	Title     string
	Status    string
	Version   string
	Type      string
	GroupsMax int
	Goal      string
	Missing   bool
	Problems  []string
}

// GroupDir is one group folder: its epic, its index file, its task files, and the group they assemble into.
type GroupDir struct {
	Epic      EpicFile
	EpicDir   string
	Dir       string
	Path      string
	TaskPaths map[string]string
	File      GroupFile
	Problems  []string
}

// Tree is docs/backlog read as epic folders holding group folders holding task files.
type Tree struct {
	Epics    []EpicFile
	Groups   []GroupDir
	Flat     []string
	Problems []string
}

// ParseEpicFile reads one the epic index file into an EpicFile without judging its content.
func ParseEpicFile(text string) EpicFile {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var file EpicFile
	file.GroupsMax = DefaultGroupsMax
	index := 0
	for index < len(lines) {
		match := epicFileHeading.FindStringSubmatch(lines[index])
		if match == nil {
			index++
			continue
		}
		file.ID, file.Title, file.Status = match[1], match[2], match[3]
		index++
		fields, _, end, err := readBlock(lines, index)
		if err != nil {
			file.Problems = append(file.Problems, err.Error())
		}
		if end >= 0 && err == nil && fields.values != nil {
			file.Version = fields.String("version")
			file.Type = fields.String("type")
			if raw := fields.String("groups_max"); raw != "" {
				n, convErr := strconv.Atoi(raw)
				if convErr != nil || n < 1 {
					file.Problems = append(file.Problems, fmt.Sprintf("groups_max %q is not a count of at least 1", raw))
				} else {
					file.GroupsMax = n
				}
			}
			index = end + 1
		}
		file.Goal = strings.TrimSpace(strings.Join(lines[index:], "\n"))
		break
	}
	if file.ID == "" {
		file.Problems = append(file.Problems, "no heading matches `## [EPIC-NN] <title> [<STATUS>]`")
	}
	return file
}

// RenderEpicFile renders an epic index file: heading, yaml block, then the goal paragraph.
func RenderEpicFile(file EpicFile) string {
	var fields Fields
	fields.Set("version", file.Version)
	if file.Type != "" {
		fields.Set("type", file.Type)
	}
	if file.GroupsMax > 0 && file.GroupsMax != DefaultGroupsMax {
		fields.Set("groups_max", strconv.Itoa(file.GroupsMax))
	}
	status := file.Status
	if status == "" {
		status = "READY"
	}
	out := fmt.Sprintf("## [%s] %s [%s]\n\n```yaml\n%s```\n", file.ID, strings.TrimSpace(file.Title), status, DumpFields(fields))
	if goal := strings.TrimSpace(file.Goal); goal != "" {
		out += "\n" + goal + "\n"
	}
	return out
}

// RenderGroupHeader renders a group index file: the group heading and its yaml block, with no version, epic or tasks.
func RenderGroupHeader(file GroupFile) string {
	var fields Fields
	fields.Set("type", file.Type)
	if file.Mode != "" {
		fields.Set("mode", file.Mode)
	}
	if file.Base != "" {
		fields.Set("base", file.Base)
	}
	fields.Set("depends_on", toAnyList(file.DependsOn))
	return RenderGroupFile(file.ID, file.Title, file.Priority, file.Status, fields)
}

// EpicDirName is the folder an epic lives in: its id in lower case.
func EpicDirName(epicID string) string { return strings.ToLower(epicID) }

// GroupDirName is the folder a group lives in: its id in lower case.
func GroupDirName(groupID string) string { return strings.ToLower(groupID) }

// TaskFileName is the file a task lives in: its id in lower case, with .md.
func TaskFileName(taskID string) string { return strings.ToLower(taskID) + ".md" }

// EpicIDOfGroup is the epic a group id belongs to by number: a group id belongs to an epic id.
func EpicIDOfGroup(groupID string) string {
	num := strings.TrimPrefix(groupID, "TG-")
	if dot := strings.Index(num, "."); dot >= 0 {
		num = num[:dot]
	}
	return "EPIC-" + num
}

// GroupIDOfTask is the group a task id belongs to by number: a task id belongs to a group id.
func GroupIDOfTask(taskID string) string {
	num := strings.TrimPrefix(taskID, "TSK-")
	if dot := strings.LastIndex(num, "."); dot >= 0 {
		num = num[:dot]
	}
	return "TG-" + num
}

// EpicDir is the absolute folder of an epic under root.
func EpicDir(root, epicID string) string {
	return filepath.Join(root, GroupFilesDir, EpicDirName(epicID))
}

// GroupDirPath is the absolute folder of a group under root, inside its epic by number.
func GroupDirPath(root, groupID string) string {
	return filepath.Join(EpicDir(root, EpicIDOfGroup(groupID)), GroupDirName(groupID))
}

// LoadTree reads every epic folder, group folder and task file under root's docs/backlog, in numeric order.
func LoadTree(root string) (Tree, error) {
	var tree Tree
	base := filepath.Join(root, GroupFilesDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return tree, nil
		}
		return tree, err
	}
	var epicDirs []string
	for _, entry := range entries {
		switch {
		case entry.IsDir() && epicDirPattern.MatchString(entry.Name()):
			epicDirs = append(epicDirs, entry.Name())
		case !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md"):
			tree.Flat = append(tree.Flat, filepath.Join(base, entry.Name()))
		}
	}
	sortNumeric(epicDirs)
	for _, name := range epicDirs {
		epicDir := filepath.Join(base, name)
		epic, err := readEpic(epicDir, name)
		if err != nil {
			return tree, err
		}
		if epic.Missing && !holdsGroupFolders(epicDir) {
			continue // an emptied folder a ship left behind is not an epic
		}
		rel := filepath.Join(GroupFilesDir, name, EpicFileName)
		for _, problem := range epic.Problems {
			tree.Problems = append(tree.Problems, rel+": "+problem)
		}
		tree.Epics = append(tree.Epics, epic)
		groupEntries, err := os.ReadDir(epicDir)
		if err != nil {
			return tree, err
		}
		var groupDirs []string
		for _, entry := range groupEntries {
			if entry.IsDir() && groupDirPattern.MatchString(entry.Name()) {
				groupDirs = append(groupDirs, entry.Name())
			}
		}
		sortNumeric(groupDirs)
		for _, groupName := range groupDirs {
			group, err := readGroupDir(epic, epicDir, filepath.Join(epicDir, groupName))
			if err != nil {
				return tree, err
			}
			if group.File.ID == "" && len(group.TaskPaths) == 0 && len(group.Problems) == 0 && len(group.File.Problems) == 0 {
				continue // an emptied folder a ship left behind is not a group
			}
			rel := filepath.Join(GroupFilesDir, name, groupName)
			for _, problem := range group.Problems {
				tree.Problems = append(tree.Problems, rel+": "+problem)
			}
			tree.Groups = append(tree.Groups, group)
		}
	}
	return tree, nil
}

// holdsGroupFolders reports whether an epic folder has any group folder with content.
func holdsGroupFolders(epicDir string) bool {
	entries, err := os.ReadDir(epicDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() || !groupDirPattern.MatchString(entry.Name()) {
			continue
		}
		inner, err := os.ReadDir(filepath.Join(epicDir, entry.Name()))
		if err == nil && len(inner) > 0 {
			return true
		}
	}
	return false
}

// readEpic parses the epic index file in dir, recording a missing or misnamed one as a problem.
func readEpic(dir, name string) (EpicFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, EpicFileName))
	if os.IsNotExist(err) {
		return EpicFile{ID: strings.ToUpper(name), GroupsMax: DefaultGroupsMax, Missing: true,
			Problems: []string{"no " + EpicFileName + " in the epic folder"}}, nil
	}
	if err != nil {
		return EpicFile{}, err
	}
	epic := ParseEpicFile(string(data))
	if epic.ID != "" && EpicDirName(epic.ID) != name {
		epic.Problems = append(epic.Problems, fmt.Sprintf("folder %s holds %s; name the folder %s", name, epic.ID, EpicDirName(epic.ID)))
	}
	return epic, nil
}

// readGroupDir assembles one group from its the group index file and task files, inheriting version and epic from the epic.
func readGroupDir(epic EpicFile, epicDir, dir string) (GroupDir, error) {
	group := GroupDir{Epic: epic, EpicDir: epicDir, Dir: dir, Path: filepath.Join(dir, GroupFileName), TaskPaths: map[string]string{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return group, err
	}
	var taskNames []string
	for _, entry := range entries {
		if !entry.IsDir() && taskFilePattern.MatchString(entry.Name()) {
			taskNames = append(taskNames, entry.Name())
		}
	}
	header, err := os.ReadFile(group.Path)
	if os.IsNotExist(err) {
		if len(taskNames) == 0 {
			return group, nil
		}
		group.Problems = append(group.Problems, "no "+GroupFileName+" in the group folder")
		group.File.ID = strings.ToUpper(filepath.Base(dir))
		return group, nil
	}
	if err != nil {
		return group, err
	}
	sortNumeric(taskNames)
	headerFile := ParseGroupFile(string(header))
	parts := []string{strings.TrimRight(string(header), "\n") + "\n"}
	for _, name := range taskNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return group, err
		}
		text := strings.TrimRight(string(data), "\n") + "\n"
		taskFile := ParseGroupFile(text)
		switch {
		case len(taskFile.Tasks) == 0:
			group.Problems = append(group.Problems, name+": no checkbox task line")
		case len(taskFile.Tasks) > 1:
			group.Problems = append(group.Problems, name+": holds more than one task")
		default:
			id := taskFile.Tasks[0].ID
			if TaskFileName(id) != name {
				group.Problems = append(group.Problems, fmt.Sprintf("%s holds %s; name the file %s", name, id, TaskFileName(id)))
			}
			group.TaskPaths[id] = filepath.Join(dir, name)
		}
		parts = append(parts, text)
	}
	file := ParseGroupFile(strings.Join(parts, "\n"))
	if headerFile.Version != "" {
		group.Problems = append(group.Problems, "version belongs in the epic's "+EpicFileName+", not "+GroupFileName)
	}
	if headerFile.EpicID != "" && headerFile.EpicID != epic.ID {
		group.Problems = append(group.Problems, fmt.Sprintf("epic %s disagrees with the folder's epic %s", headerFile.EpicID, epic.ID))
	}
	if file.ID != "" && GroupDirName(file.ID) != filepath.Base(dir) {
		group.Problems = append(group.Problems, fmt.Sprintf("folder %s holds %s; name the folder %s", filepath.Base(dir), file.ID, GroupDirName(file.ID)))
	}
	if file.ID != "" && EpicIDOfGroup(file.ID) != epic.ID {
		group.Problems = append(group.Problems, fmt.Sprintf("%s sits under %s; its number says %s", file.ID, epic.ID, EpicIDOfGroup(file.ID)))
	}
	for _, task := range file.Tasks {
		if GroupIDOfTask(task.ID) != file.ID {
			group.Problems = append(group.Problems, fmt.Sprintf("%s sits under %s; its number says %s", task.ID, file.ID, GroupIDOfTask(task.ID)))
		}
	}
	file.Version = epic.Version
	file.EpicID = epic.ID
	group.File = file
	return group, nil
}

// Locate finds the group folder whose the group index file names groupID under root.
func Locate(root, groupID string) (GroupDir, bool, error) {
	tree, err := LoadTree(root)
	if err != nil {
		return GroupDir{}, false, err
	}
	for _, group := range tree.Groups {
		if group.File.ID == groupID {
			return group, true, nil
		}
	}
	return GroupDir{}, false, nil
}

// LocateTask finds the group folder and task file that declare taskID under root.
func LocateTask(root, taskID string) (GroupDir, string, bool, error) {
	tree, err := LoadTree(root)
	if err != nil {
		return GroupDir{}, "", false, err
	}
	for _, group := range tree.Groups {
		if path, ok := group.TaskPaths[taskID]; ok {
			return group, path, true, nil
		}
	}
	return GroupDir{}, "", false, nil
}

// Group returns the tree's group with this id.
func (t Tree) Group(id string) (GroupDir, bool) {
	for _, group := range t.Groups {
		if group.File.ID == id {
			return group, true
		}
	}
	return GroupDir{}, false
}

// Epic returns the tree's epic with this id.
func (t Tree) Epic(id string) (EpicFile, bool) {
	for _, epic := range t.Epics {
		if epic.ID == id {
			return epic, true
		}
	}
	return EpicFile{}, false
}

// Files are every group of the tree as the GroupFile each assembles into, in tree order.
func (t Tree) Files() []GroupFile {
	out := make([]GroupFile, 0, len(t.Groups))
	for _, group := range t.Groups {
		out = append(out, group.File)
	}
	return out
}

// WriteEpic creates an epic folder and its the epic index file under root, refusing one that exists.
func WriteEpic(root string, epic EpicFile) (string, error) {
	dir := EpicDir(root, epic.ID)
	path := filepath.Join(dir, EpicFileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(RenderEpicFile(epic)), 0o644)
}

// WriteGroup creates a group folder, its the group index file and one file per task under the group's epic folder.
func WriteGroup(root string, file GroupFile) (string, error) {
	epicDir := EpicDir(root, EpicIDOfGroup(file.ID))
	if _, err := os.Stat(filepath.Join(epicDir, EpicFileName)); err != nil {
		return "", fmt.Errorf("no %s for %s; add the epic first", filepath.Join(GroupFilesDir, filepath.Base(epicDir), EpicFileName), file.ID)
	}
	dir := filepath.Join(epicDir, GroupDirName(file.ID))
	if _, err := os.Stat(filepath.Join(dir, GroupFileName)); err == nil {
		return dir, fmt.Errorf("%s already exists", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir, err
	}
	if err := os.WriteFile(filepath.Join(dir, GroupFileName), []byte(RenderGroupHeader(file)), 0o644); err != nil {
		return dir, err
	}
	for _, task := range file.Tasks {
		if err := os.WriteFile(filepath.Join(dir, TaskFileName(task.ID)), []byte(RenderGroupFileTask(task)), 0o644); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

// WriteTaskStatus writes one task's tick or status field into its own file.
func (g GroupDir) WriteTaskStatus(taskID, status string) error {
	path, ok := g.TaskPaths[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := SetGroupFileTaskStatus(string(data), taskID, status)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// WriteNote writes the blocker note into the group index file and flips its heading to BLOCKED.
func (g GroupDir) WriteNote(note BlockerNote) error {
	data, err := os.ReadFile(g.Path)
	if err != nil {
		return err
	}
	out, err := AddGroupFileNote(string(data), note)
	if err != nil {
		return err
	}
	return os.WriteFile(g.Path, []byte(out), 0o644)
}

// RemoveNote deletes the group index file's blocker note, reporting whether it held one.
func (g GroupDir) RemoveNote() (bool, error) {
	data, err := os.ReadFile(g.Path)
	if err != nil {
		return false, err
	}
	out, had := RemoveGroupFileNote(string(data))
	if !had {
		return false, nil
	}
	return true, os.WriteFile(g.Path, []byte(out), 0o644)
}

// SetStatus rewrites the group index file's heading status.
func (g GroupDir) SetStatus(status string) error {
	data, err := os.ReadFile(g.Path)
	if err != nil {
		return err
	}
	out, err := setGroupFileHeadingStatus(string(data), status)
	if err != nil {
		return err
	}
	return os.WriteFile(g.Path, []byte(out), 0o644)
}

// AppendTask writes a new task file with the next free id and returns the id and path.
func (g GroupDir) AppendTask(task GroupTask) (string, string, error) {
	task.ID = NextGroupFileTaskID(g.File)
	path := filepath.Join(g.Dir, TaskFileName(task.ID))
	if err := os.WriteFile(path, []byte(RenderGroupFileTask(task)), 0o644); err != nil {
		return "", "", err
	}
	return task.ID, path, nil
}

// Paths are every file of the group folder: the group index file and its task files.
func (g GroupDir) Paths() []string {
	paths := []string{g.Path}
	var tasks []string
	for _, path := range g.TaskPaths {
		tasks = append(tasks, path)
	}
	sortNumeric(tasks)
	return append(paths, tasks...)
}

// sortNumeric orders names so tg-15.2 precedes tg-15.10, comparing digit runs as numbers.
func sortNumeric(names []string) {
	sort.SliceStable(names, func(i, j int) bool { return numericLess(names[i], names[j]) })
}

// numericLess compares two names run by run, digits as numbers and the rest as text.
func numericLess(a, b string) bool {
	runsA, runsB := numberRun.FindAllString(a, -1), numberRun.FindAllString(b, -1)
	for index := 0; index < len(runsA) && index < len(runsB); index++ {
		left, right := runsA[index], runsB[index]
		if left == right {
			continue
		}
		numLeft, errLeft := strconv.Atoi(left)
		numRight, errRight := strconv.Atoi(right)
		if errLeft == nil && errRight == nil {
			return numLeft < numRight
		}
		return left < right
	}
	return len(runsA) < len(runsB)
}
