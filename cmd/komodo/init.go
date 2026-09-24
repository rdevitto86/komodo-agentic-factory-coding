package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo"
	"komodo/internal/detect"
	"komodo/internal/mount"
)

// starterRoot is where the embedded starter tree begins.
const starterRoot = "templates/project"

// runInit writes the starter files into the repo, keeping every file that already exists.
func runInit(root string, args []string) {
	set := flag.NewFlagSet("init", flag.ExitOnError)
	name := set.String("name", filepath.Base(root), "the project name the starters carry")
	_ = set.Parse(args)
	tree, err := fs.Sub(komodo.Templates, starterRoot)
	if err != nil {
		fail(err)
	}
	if err := writeStarters(os.Stdout, tree, root, starterFill(root, *name, time.Now())); err != nil {
		fail(err)
	}
	fmt.Println("\nnext:")
	fmt.Printf("  komodo install --host %s\n", mount.Names()[0])
	fmt.Println("  komodo lint")
}

// starterFill maps every placeholder the starters carry to its value for this repo.
func starterFill(root, name string, now time.Time) *strings.Replacer {
	found, _ := detect.Detect(root)
	or := func(value, fallback string) string {
		if value == "" {
			return fallback
		}
		return value
	}
	return strings.NewReplacer(
		"{{NAME}}", name,
		"<Repo Name>", name,
		"{{DATE}}", now.Format("2006-01-02"),
		"{{ONE_LINE_PURPOSE}}", "<One line: what this repo does and for whom.>",
		"{{LANGUAGE}}", or(strings.Join(found.Languages, ", "), "<language>"),
		"{{PORT}}", "<port, or none>",
		"{{ENTRYPOINT}}", "<main file or command>",
		"{{VERIFY}}", or(found.Verify, "<verify command>"),
		"{{TEST}}", "<test command>",
		"{{BUILD}}", or(found.Compile, "<build command>"),
		"{{RUN}}", "<run command>",
		"{{LINT}}", "<lint command>",
		"{{NON_OBVIOUS_PATHS}}", "- `docs/spec/`: the PRD, architecture, and system design a task's `context` cites.",
		"{{GOTCHAS}}", "None yet.",
		"{{DEVIATIONS}}", "None yet.",
	)
}

// writeStarters copies every file in tree under root, filling placeholders, dropping .tmpl, never overwriting.
func writeStarters(out io.Writer, tree fs.FS, root string, fill *strings.Replacer) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		target := strings.TrimSuffix(name, ".tmpl")
		dir, inside, err := insideDir(root, realRoot, filepath.Dir(filepath.FromSlash(target)))
		if err != nil {
			return err
		}
		if !inside {
			fmt.Fprintf(out, "skip %s: outside the repo\n", target)
			return nil
		}
		dest := filepath.Join(dir, filepath.Base(target))
		if _, err := os.Lstat(dest); err == nil {
			fmt.Fprintf(out, "keep %s\n", target)
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		data, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			fmt.Fprintf(out, "keep %s\n", target)
			return nil
		} else if err != nil {
			return err
		}
		_, err = file.WriteString(fill.Replace(string(data)))
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "create %s\n", target)
		return nil
	})
}

// insideDir creates rel under root one directory at a time, stopping at any that resolves outside realRoot.
func insideDir(root, realRoot, rel string) (string, bool, error) {
	dir := root
	if rel == "." {
		return dir, true, nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		dir = filepath.Join(dir, part)
		if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", false, err
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		} else if err != nil {
			return "", false, err
		}
		if !within(realRoot, resolved) {
			return "", false, nil
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return "", false, err
		}
		if !info.IsDir() {
			return "", false, fmt.Errorf("%s is not a directory", dir)
		}
		dir = resolved
	}
	return dir, true, nil
}

// within reports whether path is base or lies beneath it.
func within(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
