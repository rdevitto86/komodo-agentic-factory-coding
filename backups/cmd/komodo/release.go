package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/changelog"
	"komodo/internal/git"
	"komodo/internal/harness"
	"komodo/internal/release"
)

// runTag tags every changelog version no tag points at and pushes it.
func runTag(root string) {
	if err := tag(root, os.Stdout); err != nil {
		fail(err)
	}
}

// tag refuses to run off the branch origin's HEAD names, then tags and pushes every changelog version
// origin has no tag for, oldest first, each at the commit whose changelog first named it.
func tag(root string, out io.Writer) error {
	branch, err := git.Run(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if base := harness.DefaultBase(root); branch != base {
		return fmt.Errorf("refusing to tag off %q, only the default branch %q tags", branch, base)
	}
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return err
	}
	pending := release.Taggable(text, remoteTags(root))
	if len(pending) == 0 {
		fmt.Fprintln(out, "every changelog version is tagged")
		return nil
	}
	commits := introducedAt(root, pending)
	local := gitLines(root, "tag", "--list")
	for _, version := range pending {
		commit, ok := commits[version]
		if !ok {
			return fmt.Errorf("%s: no commit on %s names it in CHANGELOG.md; commit the heading first", version, branch)
		}
		name := release.TagName(version)
		if !contains(local, name) {
			if _, err := git.Run(root, "tag", "-a", name, "-m", release.TagMessage(version), commit); err != nil {
				return err
			}
		}
		if _, err := git.Run(root, "push", "origin", name); err != nil {
			return err
		}
		fmt.Fprintln(out, "tagged", name, "at", commit)
	}
	return nil
}

// introducedAt maps each version to the oldest commit in HEAD's history whose CHANGELOG.md names it.
func introducedAt(root string, versions []string) map[string]string {
	found := map[string]string{}
	for _, commit := range gitLines(root, "log", "--reverse", "--format=%H", "HEAD", "--", "CHANGELOG.md") {
		if len(found) == len(versions) {
			break
		}
		text, err := git.Run(root, "show", commit+":CHANGELOG.md")
		if err != nil {
			continue
		}
		for _, version := range versions {
			if _, ok := found[version]; !ok && release.Names(text, version) {
				found[version] = commit
			}
		}
	}
	return found
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// remoteTags lists origin's tag names, so a local tag a failed push left behind is not mistaken for shipped.
func remoteTags(root string) []string {
	out, err := git.Run(root, "ls-remote", "--tags", "origin")
	if err != nil {
		return nil
	}
	var tags []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(fields[1], "refs/tags/"), "^{}")
		if name != "" && !contains(tags, name) {
			tags = append(tags, name)
		}
	}
	return tags
}

// runRelease audits the drift between the changelog, tags and groups, builds release assets, or publishes them.
func runRelease(root string, args []string) {
	if len(args) == 0 || (args[0] != "check" && args[0] != "build" && args[0] != "publish" && args[0] != "notes") {
		fail(fmt.Errorf("usage: komodo release check | komodo release build | komodo release publish | komodo release notes EPIC-NN"))
	}
	if args[0] == "notes" {
		if len(args) != 2 {
			fail(fmt.Errorf("usage: komodo release notes EPIC-NN"))
		}
		section, err := releaseNotes(root, args[1], time.Now())
		if err != nil {
			fail(err)
		}
		fmt.Print(section)
		return
	}
	if args[0] == "publish" {
		url, err := release.Publish(root, filepath.Join(root, "dist"), os.Stdout)
		if err != nil {
			fail(err)
		}
		fmt.Println(url)
		return
	}
	if args[0] == "build" {
		paths, err := release.BuildAssets(root, filepath.Join(root, "dist"), os.Stdout)
		if err != nil {
			fail(err)
		}
		for _, path := range paths {
			fmt.Println("wrote", path)
		}
		return
	}
	drift, err := checkRelease(root)
	if err != nil {
		fail(err)
	}
	for _, item := range drift {
		fmt.Printf("%s: %s\n", item.Subject, item.Detail)
	}
	pending, err := untaggedVersions(root)
	if err != nil {
		fail(err)
	}
	for _, version := range pending {
		fmt.Printf("%s: not released, no tag on origin; the human cuts it with komodo tag on the default branch\n", version)
	}
	fmt.Printf("%d drift(s)\n", len(drift))
	if len(drift) > 0 {
		exit(1)
	}
}

// releaseNotes renders the epic's changelog section: its goal's first sentence, then each landed group's title.
func releaseNotes(root, epicID string, now time.Time) (string, error) {
	parsed, _, err := harness.LoadBacklog(root)
	if err != nil {
		return "", err
	}
	epic, ok := parsed.Epic(epicID)
	if !ok {
		return "", fmt.Errorf("no epic %s in the backlog", epicID)
	}
	if epic.Version() == "" {
		return "", fmt.Errorf("%s declares no version", epicID)
	}
	var titles []string
	for _, group := range parsed.Groups {
		if group.EpicID != epicID {
			continue
		}
		landed := len(group.Tasks) > 0
		for _, task := range group.Tasks {
			landed = landed && task.Status == "DONE"
		}
		if landed {
			titles = append(titles, strings.TrimSpace(group.Title))
		}
	}
	return changelog.Section(epic.Version(), now.Format("2006-01-02"), changelog.FirstSentence(epic.Goal), titles), nil
}

// untaggedVersions lists the changelog versions newer than every tag on origin, which are not yet released.
func untaggedVersions(root string) ([]string, error) {
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	return release.Unreleased(text, remoteTags(root)), nil
}

// checkRelease audits the changelog against the tags and every shipped group's version.
func checkRelease(root string) ([]release.Drift, error) {
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	parsed, _, err := harness.LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	return release.Check(text, gitLines(root, "tag", "--list"), release.ShippedVersions(parsed)), nil
}

// gitLines runs one git command and splits its output into lines.
func gitLines(root string, args ...string) []string {
	out, err := git.Run(root, args...)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
