package comments

import (
	_ "embed"

	"crypto/sha256"
	"encoding/hex"
)

//go:embed rules.go
var rulesSource []byte

//go:embed lint.go
var lintSource []byte

// Fingerprint hashes the lint's own rule source, so a new or changed rule always changes it.
var Fingerprint = fingerprint()

// fingerprint hashes rules.go and lint.go together into the hex digest Fingerprint holds.
func fingerprint() string {
	sum := sha256.Sum256(append(append([]byte{}, rulesSource...), lintSource...))
	return hex.EncodeToString(sum[:])
}
