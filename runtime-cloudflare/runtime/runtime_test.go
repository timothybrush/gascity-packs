package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// memWorker is an in-process Worker for unit tests: it records the last
// request and serves the session lifecycle the ops depend on.
type memWorker struct {
	mu       sync.Mutex
	sessions map[string]map[string]string // name -> meta
	created  map[string]time.Time
	lastPath string
	lastBody []byte
}

func newMemWorker() *memWorker {
	return &memWorker{sessions: map[string]map[string]string{}, created: map[string]time.Time{}}
}

func (m *memWorker) server(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(m)
	return srv, srv.Close
}

func (m *memWorker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	m.lastPath = r.URL.EscapedPath()
	m.lastBody = body
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "session":
		var req struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(body, &req)
		if _, ok := m.sessions[req.SessionID]; ok {
			w.WriteHeader(http.StatusConflict)
			return
		}
		m.sessions[req.SessionID] = map[string]string{}
		m.created[req.SessionID] = time.Now().UTC()
		w.WriteHeader(http.StatusNoContent)
	case len(parts) == 3 && parts[2] == "stop":
		if _, ok := m.sessions[parts[1]]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(m.sessions, parts[1])
		w.WriteHeader(http.StatusNoContent)
	case len(parts) == 3 && parts[2] == "status":
		if _, ok := m.sessions[parts[1]]; !ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"alive": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"alive":  true,
			"record": map[string]any{"createdAt": m.created[parts[1]].Format(time.RFC3339Nano)},
		})
	case len(parts) == 3 && parts[2] == "exec":
		_ = json.NewEncoder(w).Encode(map[string]any{"exitCode": 0, "success": true})
	case len(parts) == 3 && parts[2] == "nudge":
		w.WriteHeader(http.StatusNoContent)
	case len(parts) == 4 && parts[2] == "meta":
		s, ok := m.sessions[parts[1]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Value string `json:"value"`
			}
			_ = json.Unmarshal(body, &req)
			s[parts[3]] = req.Value
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"value": s[parts[3]]})
		case http.MethodDelete:
			delete(s, parts[3])
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// invoke runs one RPP op against the endpoint and returns exit code + stdout.
func invoke(t *testing.T, endpoint string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv(envEndpoint, endpoint)
	t.Setenv(envToken, "")
	var out, errBuf bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestProtocolNeedsNoBackend(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"protocol"}, strings.NewReader(""), &out, io.Discard)
	if code != exitOK {
		t.Fatalf("protocol exit = %d, want 0", code)
	}
	var hs struct {
		Version      int      `json:"version"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(out.Bytes(), &hs); err != nil {
		t.Fatalf("handshake not valid JSON: %v (%q)", err, out.String())
	}
	if hs.Version != 0 {
		t.Errorf("version = %d, want 0", hs.Version)
	}
	if len(hs.Capabilities) != 1 || hs.Capabilities[0] != "report-activity" {
		t.Errorf("capabilities = %v, want [report-activity]", hs.Capabilities)
	}
}

func TestMissingEndpointFailsCleanly(t *testing.T) {
	t.Setenv(envEndpoint, "")
	var errBuf bytes.Buffer
	if code := run([]string{"is-running", "s"}, strings.NewReader(""), io.Discard, &errBuf); code != exitError {
		t.Fatalf("exit = %d, want 1 (missing endpoint)", code)
	}
	if !strings.Contains(errBuf.String(), envEndpoint) {
		t.Errorf("stderr %q should name the missing env var", errBuf.String())
	}
}

func TestLifecycleRoundTrip(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()

	if code, _, errs := invoke(t, srv.URL, `{"command":"sleep 1"}`, "start", "sess"); code != exitOK {
		t.Fatalf("start exit = %d, stderr=%s", code, errs)
	}
	if code, out, _ := invoke(t, srv.URL, "", "is-running", "sess"); code != exitOK || strings.TrimSpace(out) != "true" {
		t.Fatalf("is-running after start = (%d, %q), want (0, true)", code, out)
	}
	if code, _, errs := invoke(t, srv.URL, "", "stop", "sess"); code != exitOK {
		t.Fatalf("stop exit = %d, stderr=%s", code, errs)
	}
	if code, out, _ := invoke(t, srv.URL, "", "is-running", "sess"); code != exitOK || strings.TrimSpace(out) != "false" {
		t.Fatalf("is-running after stop = (%d, %q), want (0, false)", code, out)
	}
	// Idempotent stop on the now-gone session.
	if code, _, errs := invoke(t, srv.URL, "", "stop", "sess"); code != exitOK {
		t.Fatalf("idempotent stop exit = %d, stderr=%s", code, errs)
	}
}

func TestStartForwardsConfigVerbatim(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	cfg := `{"work_dir":"/w","command":"codex exec","env":{"GC_CITY":"/c"},"process_names":["codex"]}`
	if code, _, errs := invoke(t, srv.URL, cfg, "start", "sess"); code != exitOK {
		t.Fatalf("start exit = %d, stderr=%s", code, errs)
	}
	var got struct {
		SessionID string          `json:"sessionId"`
		Config    json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(m.lastBody, &got); err != nil {
		t.Fatalf("worker body not JSON: %v", err)
	}
	if got.SessionID != "sess" {
		t.Errorf("sessionId = %q, want sess", got.SessionID)
	}
	if !bytesEqualJSON(t, []byte(cfg), got.Config) {
		t.Errorf("config forwarded = %s, want %s", got.Config, cfg)
	}
}

func TestStartRejectsNonJSONStdin(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	if code, _, errs := invoke(t, srv.URL, "not json", "start", "sess"); code != exitError {
		t.Fatalf("start with junk stdin exit = %d, want 1", code)
	} else if !strings.Contains(errs, "valid JSON") {
		t.Errorf("stderr %q should explain the JSON failure", errs)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	invoke(t, srv.URL, "", "start", "sess")
	if code, _, errs := invoke(t, srv.URL, "hello-value", "set-meta", "sess", "k"); code != exitOK {
		t.Fatalf("set-meta exit = %d, stderr=%s", code, errs)
	}
	if code, out, _ := invoke(t, srv.URL, "", "get-meta", "sess", "k"); code != exitOK || out != "hello-value" {
		t.Fatalf("get-meta = (%d, %q), want (0, hello-value)", code, out)
	}
	if code, _, _ := invoke(t, srv.URL, "", "remove-meta", "sess", "k"); code != exitOK {
		t.Fatalf("remove-meta failed")
	}
	if _, out, _ := invoke(t, srv.URL, "", "get-meta", "sess", "k"); out != "" {
		t.Errorf("get-meta after remove = %q, want empty", out)
	}
}

func TestGetMetaOnMissingSessionIsEmpty(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	if code, out, _ := invoke(t, srv.URL, "", "get-meta", "ghost", "k"); code != exitOK || out != "" {
		t.Fatalf("get-meta on missing = (%d, %q), want (0, empty)", code, out)
	}
}

func TestGetLastActivityReturnsRFC3339(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	invoke(t, srv.URL, "", "start", "sess")
	code, out, errs := invoke(t, srv.URL, "", "get-last-activity", "sess")
	if code != exitOK {
		t.Fatalf("get-last-activity exit = %d, stderr=%s", code, errs)
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(out)); err != nil {
		t.Errorf("output %q is not RFC3339Nano: %v", out, err)
	}
}

func TestProcessAliveTrueForLiveSession(t *testing.T) {
	m := newMemWorker()
	srv, done := m.server(t)
	defer done()
	invoke(t, srv.URL, "", "start", "sess")
	if code, out, _ := invoke(t, srv.URL, "codex\nnode\n", "process-alive", "sess"); code != exitOK || strings.TrimSpace(out) != "true" {
		t.Fatalf("process-alive = (%d, %q), want (0, true)", code, out)
	}
}

func TestUnknownOpExitsTwo(t *testing.T) {
	if code, _, _ := invoke(t, "http://127.0.0.1:0", "", "list-running", "prefix"); code != exitUnknown {
		t.Fatalf("list-running exit = %d, want 2 (unimplemented)", code)
	}
	if code, _, _ := invoke(t, "http://127.0.0.1:0", "", "totally-made-up"); code != exitUnknown {
		t.Fatalf("unknown op exit = %d, want 2", code)
	}
}

func TestUnimplementedAndNoBackendOpsDoNotRequireEndpoint(t *testing.T) {
	// Regression: ops that need no Worker (protocol, is-attached) and
	// unimplemented ops (list-running, attach, …) must answer without
	// GC_CLOUDFLARE_RUNTIME_URL set — otherwise `gc doctor` orphan-session
	// scans turn an unsupported list-running into a hard endpoint error.
	t.Setenv(envEndpoint, "")
	t.Setenv(envToken, "")
	cases := []struct {
		args     []string
		wantCode int
	}{
		{[]string{"protocol"}, exitOK},
		{[]string{"is-attached", "sess"}, exitOK},
		{[]string{"list-running", "prefix"}, exitUnknown},
		{[]string{"attach", "sess"}, exitUnknown},
		{[]string{"copy-to", "sess", "src", "dst"}, exitUnknown},
		{[]string{"totally-made-up"}, exitUnknown},
	}
	for _, tc := range cases {
		var out, errBuf bytes.Buffer
		if code := run(tc.args, strings.NewReader(""), &out, &errBuf); code != tc.wantCode {
			t.Errorf("run(%v) with no endpoint = %d, want %d (stderr=%q)", tc.args, code, tc.wantCode, errBuf.String())
		}
	}
}

func TestIsAttachedAlwaysFalse(t *testing.T) {
	if code, out, _ := invoke(t, "http://127.0.0.1:0", "", "is-attached", "sess"); code != exitOK || strings.TrimSpace(out) != "false" {
		t.Fatalf("is-attached = (%d, %q), want (0, false)", code, out)
	}
}

func bytesEqualJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var ai, bi any
	if err := json.Unmarshal(a, &ai); err != nil {
		t.Fatalf("a not JSON: %v", err)
	}
	if err := json.Unmarshal(b, &bi); err != nil {
		t.Fatalf("b not JSON: %v", err)
	}
	aj, _ := json.Marshal(ai)
	bj, _ := json.Marshal(bi)
	return bytes.Equal(aj, bj)
}
