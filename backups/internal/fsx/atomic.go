// Package fsx writes files so a crash or a concurrent reader never sees half of one.
package fsx

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to a temp file beside path and renames it over path, creating the directory.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
