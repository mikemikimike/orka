package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func runHostCLI(t *testing.T, handler http.HandlerFunc, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append(args, "--server", srv.URL, "--token", "person-token"))
	err := root.Execute()
	return out.String(), err
}

func TestHostStatusWakeAndDelete(t *testing.T) {
	var deleted, woken bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		host := map[string]any{"name": "gh-alice", "environment": "orka-gh-alice", "phase": "Suspended", "githubLogin": "alice", "suspended": true}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == agentHostAPIPath:
		case r.Method == http.MethodPost && r.URL.Path == agentHostAPIPath+"/wake":
			woken = true
		case r.Method == http.MethodDelete && r.URL.Path == agentHostAPIPath:
			deleted = true
			host["deleting"] = true
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(host) //nolint:errcheck
	}
	out, err := runHostCLI(t, handler, "host", "status")
	if err != nil || !strings.Contains(out, "orka-gh-alice: Suspended") || !strings.Contains(out, "orka host wake") {
		t.Fatalf("status: %v %q", err, out)
	}
	if out, err := runHostCLI(t, handler, "host", "wake"); err != nil || !woken || !strings.Contains(out, "Waking orka-gh-alice") {
		t.Fatalf("wake: %v %q", err, out)
	}
	// Deleting the volume needs an explicit yes.
	if _, err := runHostCLI(t, handler, "host", "delete"); err == nil || deleted {
		t.Fatal("delete without --yes must refuse and send nothing")
	}
	if out, err := runHostCLI(t, handler, "host", "delete", "--yes"); err != nil || !deleted || !strings.Contains(out, "Deleting orka-gh-alice") {
		t.Fatalf("delete: %v %q", err, out)
	}
}

// Linking a provider that serves agent hosts waits for the host and names
// the environment to pick in the Copilot app.
func TestConnectWaitsForTheAgentHost(t *testing.T) {
	previous, previousHost := connectionReadyPollInterval, agentHostReadyPollInterval
	connectionReadyPollInterval, agentHostReadyPollInterval = time.Millisecond, time.Millisecond
	t.Cleanup(func() { connectionReadyPollInterval, agentHostReadyPollInterval = previous, previousHost })
	origBrowser := openBrowserFunc
	openBrowserFunc = func(string) error { return nil }
	t.Cleanup(func() { openBrowserFunc = origBrowser })

	hostPolls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/connections":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"connection":   map[string]any{"name": "github-abc", "namespace": "team-a", "provider": "github", "mode": "readOnly", "state": "Pending"},
				"authorizeURL": "https://github.com/login/oauth/authorize?state=signed",
				"agentHost":    true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/connections/github-abc":
			json.NewEncoder(w).Encode(map[string]any{"name": "github-abc", "provider": "github", "mode": "readOnly", "state": "Ready", "ready": true, "grantSequence": 1}) //nolint:errcheck
		case r.Method == http.MethodGet && r.URL.Path == agentHostAPIPath:
			hostPolls++
			if hostPolls == 1 {
				// The host follows the link by a moment.
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"name": "gh-alice", "environment": "orka-gh-alice", "phase": "Ready", "ready": hostPolls >= 3}) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	out, err := runHostCLI(t, handler, "connect", "github", "--timeout", "5s")
	if err != nil {
		t.Fatalf("connect: %v %q", err, out)
	}
	if !strings.Contains(out, "Linked github") || !strings.Contains(out, "Host ready: orka-gh-alice") || hostPolls < 3 {
		t.Fatalf("output = %q, host polls = %d", out, hostPolls)
	}
}
