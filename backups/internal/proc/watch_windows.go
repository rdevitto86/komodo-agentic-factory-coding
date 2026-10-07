//go:build windows

package proc

import "errors"

// listProcs has no process table to read on this platform, so a watcher never samples a tree here.
func listProcs() (map[int]proc, error) { return nil, errors.New("no process listing on this platform") }

// killPid has nothing to kill on this platform, since listProcs reports no tree.
func killPid(pid int) {}
