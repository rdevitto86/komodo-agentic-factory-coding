// Package plan orders a group's tasks: dependency order, waves of disjoint claims, and blocked dependents.
package plan

import (
	"fmt"
	"path"
	"strings"

	"komodo/internal/backlog"
)

// Topological orders tasks so every dependency precedes its dependents, file order breaking ties.
func Topological(tasks []backlog.Task) ([]backlog.Task, error) {
	byID := map[string]backlog.Task{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	state := map[string]int{}
	var ordered []backlog.Task
	var visit func(backlog.Task, []string) error
	visit = func(task backlog.Task, trail []string) error {
		switch state[task.ID] {
		case 2:
			return nil
		case 1:
			return fmt.Errorf("dependency cycle: %s", strings.Join(append(trail, task.ID), " -> "))
		}
		state[task.ID] = 1
		for _, dep := range task.DependsOn() {
			if parent, ok := byID[dep]; ok {
				if err := visit(parent, append(trail, task.ID)); err != nil {
					return err
				}
			}
		}
		state[task.ID] = 2
		ordered = append(ordered, task)
		return nil
	}
	for _, task := range tasks {
		if err := visit(task, nil); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// Overlap reports whether two tasks' claims, test files included, share a file or nest one in a directory.
func Overlap(left, right backlog.Task) bool {
	for _, a := range claims(left) {
		for _, b := range claims(right) {
			if claimOverlaps(a, b) {
				return true
			}
		}
	}
	return false
}

// claimOverlaps reports whether two clean claims, either possibly a test-file pattern, name one file or nest.
func claimOverlaps(a, b string) bool {
	if a == b || a == "." || b == "." || strings.HasPrefix(b, a+"/") || strings.HasPrefix(a, b+"/") {
		return true
	}
	if matched, _ := path.Match(a, b); matched && strings.Contains(a, "*") {
		return true
	}
	if matched, _ := path.Match(b, a); matched && strings.Contains(b, "*") {
		return true
	}
	return false
}

// claims are a task's listed paths as clean slash paths, plus the test files a builder may touch beside them.
func claims(task backlog.Task) []string {
	var out []string
	for _, raw := range task.Files() {
		trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
		if trimmed != "" {
			clean := path.Clean(trimmed)
			out = append(out, clean)
			out = append(out, testClaims(clean)...)
		}
	}
	return out
}

// testClaims names a file's conventional tests: its package's *_test.go for Go, same-stem variants otherwise.
func testClaims(file string) []string {
	ext := path.Ext(file)
	if ext == "" || ext == path.Base(file) {
		return nil
	}
	dir, base := path.Split(file)
	stem := strings.TrimSuffix(base, ext)
	if ext == ".go" {
		return []string{path.Join(dir, stem+"_test.go"), path.Join(dir, "*_test.go")}
	}
	var out []string
	for _, name := range []string{stem + ".test" + ext, stem + ".spec" + ext, "test_" + stem + ext, stem + "_test" + ext} {
		out = append(out, path.Join(dir, name))
	}
	return out
}

// Waves groups tasks so members of one wave share no claimed file and no unmet dependency.
func Waves(tasks []backlog.Task, done []string, capacity int) ([][]backlog.Task, error) {
	ordered, err := Topological(tasks)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, task := range tasks {
		known[task.ID] = true
	}
	finished := map[string]bool{}
	for _, id := range done {
		finished[id] = true
	}
	var pending []backlog.Task
	for _, task := range ordered {
		if !finished[task.ID] {
			pending = append(pending, task)
		}
	}
	var result [][]backlog.Task
	for len(pending) > 0 {
		var wave, remaining []backlog.Task
		for _, task := range pending {
			ready := true
			for _, dep := range task.DependsOn() {
				if known[dep] && !finished[dep] {
					ready = false
					break
				}
			}
			clash := false
			for _, member := range wave {
				if Overlap(task, member) {
					clash = true
					break
				}
			}
			if ready && !clash && (capacity <= 0 || len(wave) < capacity) {
				wave = append(wave, task)
				continue
			}
			remaining = append(remaining, task)
		}
		if len(wave) == 0 {
			var ids []string
			for _, task := range remaining {
				ids = append(ids, task.ID)
			}
			return nil, fmt.Errorf("no runnable task among: %s", strings.Join(ids, ", "))
		}
		for _, task := range wave {
			finished[task.ID] = true
		}
		result = append(result, wave)
		pending = remaining
	}
	return result, nil
}

// BlockedBy lists every task that transitively depends on the failed one.
func BlockedBy(tasks []backlog.Task, failed string) []string {
	dependents := map[string][]string{}
	for _, task := range tasks {
		for _, dep := range task.DependsOn() {
			dependents[dep] = append(dependents[dep], task.ID)
		}
	}
	seen := map[string]bool{}
	var out []string
	frontier := []string{failed}
	for len(frontier) > 0 {
		current := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		for _, child := range dependents[current] {
			if !seen[child] {
				seen[child] = true
				out = append(out, child)
				frontier = append(frontier, child)
			}
		}
	}
	return out
}
