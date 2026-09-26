package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/detect"
	"komodo/internal/facet"
	repopkg "komodo/internal/repo"
)

// queueCard is the subset of a compiled ingest card a brief reads from disk, never importing ingest.
type queueCard struct {
	Files   []string `json:"files"`
	Context []string `json:"context"`
}

// loadCard reads a group's compiled card from .komodo/queue, or reports it missing.
func loadCard(root, groupID string) (queueCard, bool) {
	data, err := os.ReadFile(filepath.Join(root, StateDir, "queue", groupID+".json"))
	if err != nil {
		return queueCard{}, false
	}
	var card queueCard
	if json.Unmarshal(data, &card) != nil {
		return queueCard{}, false
	}
	return card, true
}

// cardTask overrides a task's files and context with its group's compiled card, when one has
// already been written, so a brief consumes ingest's expanded list instead of re-deriving it.
func cardTask(root string, task backlog.Task, groupID string) backlog.Task {
	card, ok := loadCard(root, groupID)
	if !ok {
		return task
	}
	if len(card.Files) > 0 {
		task.Fields.Set("files", toAnyList(card.Files))
	}
	if len(card.Context) > 0 {
		task.Fields.Set("context", toAnyList(card.Context))
	}
	return task
}

// toAnyList wraps a string slice for Fields.Set, which stores list values as []any.
func toAnyList(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

// repoRules is the repo's own AGENTS.md, clipped, or a one-line default.
func repoRules(cwd string, limit int) string {
	data, err := os.ReadFile(filepath.Join(cwd, "AGENTS.md"))
	if err == nil {
		return Clip(string(data), limit, "AGENTS.md")
	}
	return "No repo-level rules file. Follow the standards below and the code's existing idioms."
}

// repoContextSlot renders every repo context file whose paths match the task's files, clipped to
// the slot's total, since the README names one cap for the whole joined slot, not one per file.
func repoContextSlot(cwd string, task backlog.Task, limit int) string {
	contexts, _ := repopkg.LoadContext(cwd)
	var parts []string
	for _, context := range contexts {
		if context.Matches(task.Files()) {
			parts = append(parts, context.Body)
		}
	}
	if len(parts) == 0 {
		return "None declared for this repo."
	}
	return Clip(strings.Join(parts, "\n\n---\n\n"), limit, "repo context")
}

// contextSlot resolves each context anchor to its section, clipped per file and by the joined total.
func contextSlot(cwd string, task backlog.Task, perFileCap, totalCap int) string {
	var parts []string
	for _, ref := range task.Context() {
		path, anchor, _ := strings.Cut(ref, "#")
		data, err := os.ReadFile(filepath.Join(cwd, path))
		if err != nil {
			parts = append(parts, fmt.Sprintf("### %s\n[%s does not exist yet]", ref, path))
			continue
		}
		text := string(data)
		if anchor != "" {
			section := Section(text, anchor)
			if section == "" {
				// A mistyped anchor names its miss instead of spending the slot on the whole file.
				parts = append(parts, fmt.Sprintf("### %s\n[%s has no section %q; komodo lint reports it]", ref, path, anchor))
				continue
			}
			text = section
		}
		parts = append(parts, fmt.Sprintf("### %s\n%s", ref, Clip(text, perFileCap, ref)))
	}
	if len(parts) == 0 {
		return "None beyond the files below."
	}
	return Clip(strings.Join(parts, "\n\n"), totalCap, "context")
}

// filesSlot reads every listed file, names a binary one by size, and never reads bin/, clipped
// per file and by the joined total, since a per-file floor alone lets many files past it.
func filesSlot(cwd string, task backlog.Task, perFileCap, totalCap int) string {
	files := task.Files()
	if len(files) == 0 {
		return "No files listed."
	}
	perFile := perFileCap
	if perFile*len(files) > totalCap {
		perFile = totalCap / len(files)
		if perFile < 2000 {
			perFile = 2000
		}
	}
	var parts []string
	for _, path := range files {
		full := filepath.Join(cwd, path)
		info, err := os.Stat(full)
		switch {
		case err != nil:
			parts = append(parts, fmt.Sprintf("### %s\n[does not exist yet]", path))
		case info.IsDir():
			parts = append(parts, fmt.Sprintf("### %s\n[a directory, %s]", path, path))
		case strings.HasPrefix(strings.ReplaceAll(path, "\\", "/"), "bin/"):
			parts = append(parts, fmt.Sprintf("### %s\n[a prebuilt binary, %d bytes, never read]", path, info.Size()))
		case !IsText(full):
			parts = append(parts, fmt.Sprintf("### %s\n[not text, %d bytes, never read]", path, info.Size()))
		default:
			data, err := os.ReadFile(full)
			if err != nil {
				parts = append(parts, fmt.Sprintf("### %s\n[unreadable: %v]", path, err))
				continue
			}
			parts = append(parts, fmt.Sprintf("### %s\n```\n%s\n```", path, Clip(string(data), perFile, path)))
		}
	}
	return Clip(strings.Join(parts, "\n\n"), totalCap, "files")
}

// standardsSlot renders each selected standard, clipped by its own cap.
func standardsSlot(selected []Standard) string {
	if len(selected) == 0 {
		return "No language standard matches these files."
	}
	var parts []string
	for _, standard := range selected {
		parts = append(parts, Clip(strings.TrimSpace(standard.Body), CapStandard, standard.Name))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// repoProfileSlot summarises the repo's own detected profile: languages, cloud, data, CI, and verify.
func repoProfileSlot(profile detect.Profile) string {
	fields := []string{
		"languages: " + joinOrNone(profile.Languages),
		"cloud: " + joinOrNone(profile.Cloud),
		"data: " + joinOrNone(profile.Data),
		"ci: " + joinOrNone(profile.CI),
		"verify: " + joinOrNone(nonEmpty(profile.Verify)),
	}
	return Clip(strings.Join(fields, "; "), CapRepoProfile, "repo profile")
}

// joinOrNone joins a list with commas, or names it none when empty.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ",")
}

// nonEmpty wraps a single string into a one-item list, dropping it when empty.
func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// facetAppendixSlot renders the appendix each selected facet carries for this role, its own cap.
func facetAppendixSlot(root string, profile detect.Profile, task backlog.Task, role string) string {
	names, err := facet.Select(root, profile, task.Facets())
	if err != nil {
		return ""
	}
	var parts []string
	for _, name := range names {
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		appendix := loaded.BuilderAppendix()
		if role == "reviewer" {
			appendix = loaded.ReviewerAppendix()
		}
		if appendix == "" {
			continue
		}
		parts = append(parts, Clip(appendix, CapFacet, loaded.Name))
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n---\n\n" + strings.Join(parts, "\n\n---\n\n")
}

// doneWhenSlot lists the commands whose zero exit proves the task done.
func doneWhenSlot(task backlog.Task) string {
	var lines []string
	for _, command := range task.DoneWhen() {
		lines = append(lines, "- `"+command+"`")
	}
	return strings.Join(lines, "\n")
}

// failureSlot carries the previous attempt into a repair brief, clipped.
func failureSlot(failure string, limit int) string {
	if strings.TrimSpace(failure) == "" {
		return ""
	}
	return "\n# Previous attempt failed\nFix the cause. Never weaken the check.\n```\n" +
		Clip(failure, limit, "failure") + "\n```"
}
