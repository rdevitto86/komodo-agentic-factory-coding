// Package line is the conveyor: intake, the input and output devices, QC, and ship.
package line

import (
	"fmt"
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

// dirsOverlap reports whether two tasks claim the same directory, or one claims a parent of the other's.
func dirsOverlap(left, right backlog.Task) bool {
	for _, a := range left.Dirs() {
		for _, b := range right.Dirs() {
			if a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
				return true
			}
		}
	}
	return false
}

// Waves groups tasks so members of one wave share no directory and no unmet dependency.
func Waves(tasks []backlog.Task, done []string) ([][]backlog.Task, error) {
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
				if dirsOverlap(task, member) {
					clash = true
					break
				}
			}
			if ready && !clash {
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
