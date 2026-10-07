package fsx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// ReadStrictJSON decodes path into out strictly; an absent or blank file reports not found, a bad one names path.
func ReadStrictJSON(path string, out any) (found bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return DecodeStrictJSON(path, data, out)
}

// DecodeStrictJSON decodes data read from path into out, refusing unknown fields and trailing data; blank is not found.
func DecodeStrictJSON(path string, data []byte, out any) (found bool, err error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("%s: data after the JSON value", path)
	}
	return true, nil
}
