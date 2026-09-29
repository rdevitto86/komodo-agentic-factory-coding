package claude

import (
	"os"
	"path/filepath"
	"testing"
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
