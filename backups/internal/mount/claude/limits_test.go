package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProbeReadsExtraUsageAndTheBillingType(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	body := `{"oauthAccount":{"organizationRateLimitTier":"default_claude_max_5x",` +
		`"hasExtraUsageEnabled":true,"billingType":"metered"}}`
	if err := os.WriteFile(filepath.Join(home, configFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	usage, ok := Probe()
	if !ok || usage.Plan != "max_5x" {
		t.Fatalf("usage = %+v, ok = %v", usage, ok)
	}
	if !usage.ExtraUsage || usage.BillingType != "metered" {
		t.Fatalf("usage = %+v; extra usage and billing type did not read", usage)
	}
}

func TestLoggedInReadsTheAuthStatusReport(t *testing.T) {
	previous := authStatus
	t.Cleanup(func() { authStatus = previous })
	for _, c := range []struct {
		report string
		want   bool
	}{
		{`{"loggedIn": true, "authMethod": "claude.ai"}`, true},
		{`{"loggedIn": true, "authMethod": "api_key"}`, true},
		{`{"loggedIn": false, "authMethod": "none"}`, false},
	} {
		authStatus = func() ([]byte, error) { return []byte(c.report), nil }
		if got, err := LoggedIn(); err != nil || got != c.want {
			t.Errorf("LoggedIn(%s) = %v, %v; want %v", c.report, got, err, c.want)
		}
	}
}

// TestAuthStatusKillsAHungClaudeAndNamesTheTimeout proves claude auth status past cliTimeout is
// killed, process group included, and the error names the command and the timeout.
func TestAuthStatusKillsAHungClaudeAndNamesTheTimeout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	saved := cliTimeout
	cliTimeout = 200 * time.Millisecond
	t.Cleanup(func() { cliTimeout = saved })
	started := time.Now()
	if _, err := authStatus(); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}
