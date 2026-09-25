package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/line"
	"komodo/internal/release"
)

// runTag tags every changelog version no tag points at and pushes it.
func runTag(root string) {
	if err := tag(root, os.Stdout); err != nil {
		fail(err)
	}
}

// tag refuses to run off the branch origin's HEAD names, then tags and pushes every version origin lacks.
func tag(root string, out io.Writer) error {
	branch, err := git.Run(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if base := line.DefaultBase(root); branch != base {
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
	local := gitLines(root, "tag", "--list")
	for _, version := range pending {
		name := release.TagName(version)
		if !contains(local, name) {
			if _, err := git.Run(root, "tag", "-a", name, "-m", release.TagMessage(version)); err != nil {
				return err
			}
		}
		if _, err := git.Run(root, "push", "origin", name); err != nil {
			return err
		}
		fmt.Fprintln(out, "tagged", name)
	}
	return nil
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

// runRelease audits the drift between the changelog, the tags, and the groups, or builds release assets.
func runRelease(root string, args []string) {
	if len(args) == 0 || (args[0] != "check" && args[0] != "build") {
		fail(fmt.Errorf("usage: komodo release check | komodo release build"))
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

// untaggedVersions lists the changelog versions origin holds no tag for, which are not yet released.
func untaggedVersions(root string) ([]string, error) {
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	return release.Taggable(text, remoteTags(root)), nil
}

// checkRelease audits the changelog against the tags and every shipped group's version.
func checkRelease(root string) ([]release.Drift, error) {
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	_, parsed := load(root)
	var versions []string
	for _, group := range parsed.Groups {
		if !shipped(group) {
			continue
		}
		versions = append(versions, group.Version())
	}
	return release.Check(text, gitLines(root, "tag", "--list"), versions), nil
}

// shipped reports whether every task in the group is DONE.
func shipped(group backlog.Group) bool {
	if len(group.Tasks) == 0 {
		return false
	}
	for _, task := range group.Tasks {
		if task.Status != "DONE" {
			return false
		}
	}
	return true
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
