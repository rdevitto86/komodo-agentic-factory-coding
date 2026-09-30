package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"komodo/internal/line"
	"komodo/internal/pr"
)

// runRephase parses komodo rephase <epic> <new-version> and runs it against this repo's origin.
func runRephase(root string, args []string) {
	if len(args) < 2 {
		fail(fmt.Errorf("usage: komodo rephase <epic> <new-version>"))
	}
	if err := rephase(root, args[0], args[1], pr.New(root), os.Stdin, os.Stdout); err != nil {
		fail(err)
	}
}

// rephase moves epic to newVersion, prints what it did, then asks in before deleting the old branch.
func rephase(root, epic, newVersion string, client *pr.Client, in io.Reader, out io.Writer) error {
	result, err := line.Rephase(root, epic, newVersion, client)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s moved from %s to %s\n", epic, result.OldVersion, result.NewVersion)
	fmt.Fprintf(out, "pushed %s from %s\n", result.NewBranch, result.OldBranch)
	for _, url := range result.Retargeted {
		fmt.Fprintf(out, "retargeted %s to %s\n", url, result.NewBranch)
	}
	fmt.Fprintf(out, "delete %s? [y/N] ", result.OldBranch)
	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	if !isYes(answer) {
		fmt.Fprintf(out, "kept %s\n", result.OldBranch)
		return nil
	}
	if err := line.DeleteBranch(root, result.OldBranch); err != nil {
		return err
	}
	fmt.Fprintf(out, "deleted %s\n", result.OldBranch)
	return nil
}

// isYes reports whether a prompt's answer is y or yes, case-insensitively.
func isYes(answer string) bool {
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
