package check

import (
	"fmt"
	"regexp"
)

// secretPattern is one named regular expression for a common key or token shape.
type secretPattern struct {
	name    string
	pattern *regexp.Regexp
}

// secretPatterns are the standard-library patterns Secrets scans added lines for.
var secretPatterns = []secretPattern{
	{"AWS access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"AWS secret key", regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*['"]?[A-Za-z0-9/+=]{40}['"]?`)},
	{"GitHub token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"Slack token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"private key block", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
	{"generic API key", regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['"][A-Za-z0-9_\-/+=]{16,}['"]`)},
}

// Secrets scans a diff's added lines for a common key or token, naming each hit's file, line, and pattern.
func Secrets(diff string) []string {
	var problems []string
	for _, line := range ParseAddedLines(diff) {
		for _, p := range secretPatterns {
			if p.pattern.MatchString(line.Text) {
				problems = append(problems, fmt.Sprintf("secret: %s:%d looks like a %s", line.File, line.Line, p.name))
			}
		}
	}
	return problems
}
