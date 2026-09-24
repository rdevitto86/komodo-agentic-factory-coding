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
	return fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		target := strings.TrimSuffix(name, ".tmpl")
		dest := filepath.Join(root, filepath.FromSlash(target))
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
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(fill.Replace(string(data))), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "create %s\n", target)
		return nil
	})
}
