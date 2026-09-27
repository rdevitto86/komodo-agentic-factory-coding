package review

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/check"
	"komodo/internal/proc"
)

// Finding is one lens finding as the reviewer's schema describes it.
type Finding struct {
	Lens     string `json:"lens"`
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Class    string `json:"class"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence"`
	Fix      string `json:"fix"`
}

// Tree is what a finding's evidence is checked against: the worktree, its diff, and the validators' report.
type Tree struct {
	Worktree string
	Diff     string
	Report   Report
}

// Checked is one finding and whether its evidence held; a finding that does not block becomes a PR note.
type Checked struct {
	Finding Finding
	Blocks  bool
	Why     string
}

// evidenceKind is the proof a finding's class needs before it blocks.
type evidenceKind int

// The three kinds of evidence the binary can verify.
const (
	reproducerFails evidenceKind = iota + 1
	ruleOnChangedLine
	validatorMeasured
)

// evidenceByClass maps each finding class to the evidence it needs; a class missing here never blocks.
var evidenceByClass = map[string]evidenceKind{
	"bug": reproducerFails, "security": reproducerFails,
	"convention": ruleOnChangedLine, "simplify": ruleOnChangedLine, "narrative-comment": ruleOnChangedLine,
	"undocumented-nonobvious": ruleOnChangedLine, "test-gap": ruleOnChangedLine,
	"performance": validatorMeasured, "blast-radius": validatorMeasured,
}

// ruleLens maps every checklist rule ID to the lens whose skill lists it.
var ruleLens = map[string]string{
	"COR-1": "correctness", "COR-2": "correctness", "COR-3": "correctness", "COR-4": "correctness",
	"COR-5": "correctness",
	"SEC-1": "security", "SEC-2": "security", "SEC-3": "security", "SEC-4": "security",
	"RDY-1": "security", "RDY-2": "security", "RDY-3": "security",
	"QUA-1": "quality", "QUA-2": "quality", "QUA-3": "quality", "QUA-4": "quality", "QUA-5": "quality",
}

// Exit codes a shell reports when the command itself could not run.
const (
	exitNotExecutable = 126
	exitNotFound      = 127
)

// Verify checks each finding's evidence against the tree, keeping a finding as blocking only when it holds.
func Verify(tree Tree, findings []Finding) ([]Checked, error) {
	added := map[string]map[int]bool{}
	for _, line := range check.ParseAddedLines(tree.Diff) {
		if added[line.File] == nil {
			added[line.File] = map[int]bool{}
		}
		added[line.File][line.Line] = true
	}
	out := make([]Checked, 0, len(findings))
	for _, finding := range findings {
		checked := Checked{Finding: finding}
		switch evidenceByClass[finding.Class] {
		case reproducerFails:
			blocks, why, err := reproduces(tree.Worktree, finding.Evidence)
			if err != nil {
				return nil, err
			}
			checked.Blocks, checked.Why = blocks, why
		case ruleOnChangedLine:
			checked.Blocks, checked.Why = citesRuleOnChangedLine(finding, added)
		case validatorMeasured:
			checked.Blocks, checked.Why = citesMeasurement(finding.Evidence, tree.Report)
		default:
			checked.Why = fmt.Sprintf("class %q needs no evidence the binary can verify", finding.Class)
		}
		out = append(out, checked)
	}
	return out, nil
}

// reproduces runs a reproducer command in a scratch copy of the worktree and reports whether it fails there.
func reproduces(worktree, command string) (bool, string, error) {
	if strings.TrimSpace(command) == "" {
		return false, "no reproducer command in evidence", nil
	}
	scratch, err := os.MkdirTemp("", "komodo-reproducer-")
	if err != nil {
		return false, "", err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := copyTree(worktree, scratch); err != nil {
		return false, "", err
	}
	ran := proc.Shell(scratch, command, check.CommandTimeout)
	switch {
	case ran.OK():
		return false, "the reproducer passes on the current tree", nil
	case ran.TimedOut, ran.ExitCode == exitNotExecutable, ran.ExitCode == exitNotFound:
		return false, fmt.Sprintf("the reproducer did not run: %v", ran.Err()), nil
	}
	return true, fmt.Sprintf("the reproducer fails on the current tree: %v", ran.Err()), nil
}

// copyTree copies src's files, directories and symlinks into dst, leaving out git and line state.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case entry.IsDir():
			if rel != "." && (entry.Name() == ".git" || entry.Name() == ".komodo") {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case !entry.Type().IsRegular():
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// citesRuleOnChangedLine reports whether a finding cites its lens's checklist rule on a line the diff added.
func citesRuleOnChangedLine(finding Finding, added map[string]map[int]bool) (bool, string) {
	lens, known := ruleLens[finding.RuleID]
	switch {
	case !known:
		return false, fmt.Sprintf("%q is no checklist rule ID", finding.RuleID)
	case lens != finding.Lens:
		return false, fmt.Sprintf("%s belongs to the %s lens, not %q", finding.RuleID, lens, finding.Lens)
	case !added[finding.File][finding.Line]:
		return false, fmt.Sprintf("%s:%d is not on a changed line", finding.File, finding.Line)
	}
	return true, fmt.Sprintf("%s on changed line %s:%d", finding.RuleID, finding.File, finding.Line)
}

// citesMeasurement reports whether evidence quotes a line of some validator's measurement.
func citesMeasurement(evidence string, report Report) (bool, string) {
	for _, each := range report.Measurements {
		for _, line := range strings.Split(each.Detail, "\n") {
			if line = strings.TrimSpace(line); line != "" && strings.Contains(evidence, line) {
				return true, fmt.Sprintf("the %s validator measured it: %s", each.Kind, line)
			}
		}
	}
	return false, "the evidence quotes no validator's measurement"
}
