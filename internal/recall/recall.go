// Package recall scores a reviewer machine against a corpus of seeded bugs and clean diffs.
package recall

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/toolkit"
)

// Window is how many lines a finding may sit from the seeded bug and still count as a catch.
const Window = 5

//go:embed testdata/*.diff testdata/*.json
var corpus embed.FS

// Case is one seeded diff: the file, line, and class of its bug, or Clean when it holds none.
type Case struct {
	Name  string `json:"-"`
	Diff  string `json:"-"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Class string `json:"class"`
	Clean bool   `json:"clean"`
}

// Post sends one brief and its schema to a model and returns the parsed answer.
type Post func(model, brief string, schema []byte) (mount.LocalResult, error)

// Score is one model's run over the corpus.
type Score struct {
	Caught         int       `json:"-"`
	Cases          int       `json:"cases"`
	Recall         float64   `json:"recall"`
	FalsePositives int       `json:"false_positives"`
	At             time.Time `json:"at"`
}

// Finding is one reviewer finding, trimmed to what scoring reads.
type Finding struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// counted are the severities that score: medium and above.
var counted = map[string]bool{"critical": true, "high": true, "medium": true}

// Cases reads the embedded corpus, one case per diff, sorted by name.
func Cases() ([]Case, error) {
	diffs, err := fs.Glob(corpus, "testdata/*.diff")
	if err != nil {
		return nil, err
	}
	sort.Strings(diffs)
	cases := make([]Case, 0, len(diffs))
	for _, name := range diffs {
		diff, err := corpus.ReadFile(name)
		if err != nil {
			return nil, err
		}
		meta, err := corpus.ReadFile(strings.TrimSuffix(name, ".diff") + ".json")
		if err != nil {
			return nil, err
		}
		var each Case
		if err := json.Unmarshal(meta, &each); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if !each.Clean && (each.File == "" || each.Line <= 0) {
			return nil, fmt.Errorf("%s names no file and line and is not clean", name)
		}
		each.Name = strings.TrimSuffix(path.Base(name), ".diff")
		each.Diff = strings.TrimRight(string(diff), "\n")
		cases = append(cases, each)
	}
	return cases, nil
}

// Brief fills the reviewer role for one case the way the line fills it for a group's diff.
func Brief(root string, each Case) (string, error) {
	definition, err := line.LoadRole(root, "reviewer")
	if err != nil {
		return "", err
	}
	standards, err := line.LoadStandards(root)
	if err != nil {
		return "", err
	}
	files := diffFiles(each.Diff)
	parts := make([]string, 0, len(standards))
	for _, standard := range line.StandardsFor(standards, files, "reviewer") {
		parts = append(parts, line.Clip(strings.TrimSpace(standard.Body), line.CapStandard, standard.Name))
	}
	standardsText := strings.Join(parts, "\n\n---\n\n")
	if standardsText == "" {
		standardsText = "No language standard matches these files."
	}
	return line.Fill(definition.Body, map[string]string{
		"group_id":  each.Name,
		"title":     "change to " + strings.Join(files, ", "),
		"tasks":     "Change " + strings.Join(files, ", ") + " as the diff shows.",
		"standards": standardsText,
		"diff":      each.Diff,
		"base":      "main",
	})
}

// diffFiles lists the post-change paths a diff's +++ headers name.
func diffFiles(diff string) []string {
	var files []string
	for _, row := range strings.Split(diff, "\n") {
		if name, ok := strings.CutPrefix(row, "+++ b/"); ok {
			files = append(files, name)
		}
	}
	return files
}

// Findings reads the reviewer's findings out of its parsed answer, skipping any it cannot read.
func Findings(value map[string]any) []Finding {
	data, err := json.Marshal(value["findings"])
	if err != nil {
		return nil
	}
	var raw []json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	findings := make([]Finding, 0, len(raw))
	for _, item := range raw {
		var finding Finding
		if json.Unmarshal(item, &finding) == nil {
			findings = append(findings, finding)
		}
	}
	return findings
}

// Catches reports whether a finding at medium or above names the case's file within Window lines.
func Catches(each Case, findings []Finding) bool {
	for _, finding := range findings {
		if !counted[strings.ToLower(finding.Severity)] || !sameFile(finding.File, each.File) {
			continue
		}
		distance := finding.Line - each.Line
		if distance < 0 {
			distance = -distance
		}
		if finding.Line > 0 && distance <= Window {
			return true
		}
	}
	return false
}

// sameFile compares two repo paths, ignoring a diff's a/ or b/ prefix and a leading ./.
func sameFile(got, want string) bool {
	got = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(got)), "./")
	for _, prefix := range []string{"a/", "b/"} {
		if strings.HasPrefix(got, prefix) && !strings.HasPrefix(want, prefix) {
			got = strings.TrimPrefix(got, prefix)
		}
	}
	return got == want
}

// Run posts every case's brief to model and scores the answers: recall over the bug cases, and
// the medium-or-above findings the clean cases drew.
func Run(root, model string, post Post) (Score, error) {
	cases, err := Cases()
	if err != nil {
		return Score{}, err
	}
	schema, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", "reviewer.schema.json"))
	if err != nil {
		return Score{}, err
	}
	var score Score
	for _, each := range cases {
		brief, err := Brief(root, each)
		if err != nil {
			return Score{}, err
		}
		result, err := post(model, brief, schema)
		if err != nil {
			return Score{}, fmt.Errorf("%s: %w", each.Name, err)
		}
		findings := Findings(result.Value)
		if each.Clean {
			for _, finding := range findings {
				if counted[strings.ToLower(finding.Severity)] {
					score.FalsePositives++
				}
			}
			continue
		}
		score.Cases++
		if Catches(each, findings) {
			score.Caught++
		}
	}
	if score.Cases > 0 {
		score.Recall = float64(score.Caught) / float64(score.Cases)
	}
	score.At = time.Now().UTC()
	return score, nil
}

// Save writes score under model in the file at file, keeping every other model's score.
func Save(file, model string, score Score) error {
	if file == "" {
		return errors.New("no home directory to hold recall.json")
	}
	scores := map[string]Score{}
	data, err := os.ReadFile(file)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &scores); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	scores[model] = score
	out, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, append(out, '\n'), 0o644)
}
