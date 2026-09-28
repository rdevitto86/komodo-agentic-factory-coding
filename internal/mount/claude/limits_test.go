package claude

import "testing"

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
