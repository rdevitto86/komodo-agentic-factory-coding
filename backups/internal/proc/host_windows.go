//go:build windows

package proc

import "errors"

// startTable has no process listing on this platform, so a claim falls back to its clock.
func startTable() (map[int]started, error) {
	return nil, errors.New("no process listing on this platform")
}
