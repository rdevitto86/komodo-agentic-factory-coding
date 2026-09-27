package check

import (
	"fmt"
	"strings"
	"testing"
)

// diffAdding wraps one added line's text in a minimal unified diff so Secrets can scan it.
func diffAdding(text string) string {
	return fmt.Sprintf("--- a/config.go\n+++ b/config.go\n@@ -1,0 +1,1 @@\n+%s\n", text)
}

func TestSecretsCatchesEachPattern(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"AWS access key", "key := \"AKIA" + "ABCDEFGHIJKLMNOP\""},
		{"AWS secret key", "aws_secret_access_key = \"" + strings.Repeat("Ab1/", 10) + "\""},
		{"GitHub token", "token := \"ghp_" + strings.Repeat("a", 36) + "\""},
		{"Slack token", "webhook := \"xox" + "b-1234567890-abcdefghij\""},
		{"private key block", "-----BEGIN RSA " + "PRIVATE KEY-----"},
		{"generic API key", `api_key = "` + "fixture_" + strings.Repeat("x", 16) + `"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := Secrets(diffAdding(tc.line))
			if len(problems) != 1 {
				t.Fatalf("problems = %v, want exactly one hit for %s", problems, tc.name)
			}
			if !strings.Contains(problems[0], tc.name) {
				t.Fatalf("problems[0] = %q, want it to name %q", problems[0], tc.name)
			}
			if !strings.Contains(problems[0], "config.go:1") {
				t.Fatalf("problems[0] = %q, want it to name config.go:1", problems[0])
			}
		})
	}
}

func TestSecretsIgnoresOrdinaryAddedLines(t *testing.T) {
	if problems := Secrets(diffAdding("func main() { fmt.Println(\"hello\") }")); len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestSecretsIgnoresRemovedLines(t *testing.T) {
	diff := "--- a/config.go\n+++ b/config.go\n@@ -1,1 +1,0 @@\n-key := \"AKIA" + "ABCDEFGHIJKLMNOP\"\n"
	if problems := Secrets(diff); len(problems) != 0 {
		t.Fatalf("problems = %v, want none for a removed line", problems)
	}
}
