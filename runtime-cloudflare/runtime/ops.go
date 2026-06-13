package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// opStart creates a remote session. The RPP start-config JSON arrives on
// stdin and is forwarded verbatim as the Worker's session config, so the
// Worker keeps receiving the exact payload the in-tree provider sent.
func (c *client) opStart(name string, stdin []byte) error {
	req := startRequest{SessionID: name}
	if trimmed := bytes.TrimSpace(stdin); len(trimmed) > 0 {
		if !json.Valid(trimmed) {
			return fmt.Errorf("start config on stdin is not valid JSON")
		}
		req.Config = json.RawMessage(trimmed)
	}
	return c.do(context.Background(), c.startTimeout, http.MethodPost, []string{"session"}, req, nil)
}

// opStop destroys the named session. A missing session is already stopped,
// so 404 is success — stop stays idempotent (RPP requires stop on a gone
// session to exit 0).
func (c *client) opStop(name string) error {
	return c.idempotent(c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "stop"}, nil, nil))
}

// opInterrupt best-effort SIGINTs user-owned processes in the session.
// Targets the current user only so it never signals shared-container
// system daemons.
func (c *client) opInterrupt(name string) error {
	_, err := c.exec(name, `pkill -INT -u "$(id -u)" 2>/dev/null; true`)
	return c.idempotent(err)
}

// opIsRunning reports whether the session is alive. Any error reads as not
// running, matching the in-tree provider.
func (c *client) opIsRunning(name string) bool {
	var out sessionStatusResponse
	if err := c.do(context.Background(), c.timeout, http.MethodGet, []string{"session", name, "status"}, nil, &out); err != nil {
		return false
	}
	return out.Alive
}

// opGetLastActivity returns the session creation time from the status
// record. Backs the report-activity capability.
func (c *client) opGetLastActivity(name string) (time.Time, error) {
	var out sessionStatusResponse
	if err := c.do(context.Background(), c.timeout, http.MethodGet, []string{"session", name, "status"}, nil, &out); err != nil {
		return time.Time{}, err
	}
	ts := strings.TrimSpace(out.Record.CreatedAt)
	if ts == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing cloudflare last activity for %q: %w", name, err)
	}
	return t, nil
}

// opProcessAlive reports whether any of the named processes are alive via a
// pgrep exec. Empty list reads as alive. Uses -E so the | alternation works
// and -- so process names starting with - are not parsed as flags.
func (c *client) opProcessAlive(name string, processNames []string) bool {
	if len(processNames) == 0 {
		return true
	}
	pattern := shellQuoteSingle(strings.Join(processNames, "|"))
	out, err := c.exec(name, "pgrep -Ef -- "+pattern)
	if err != nil {
		return false
	}
	return out.ExitCode == 0
}

// opNudge sends text to the session. Empty text is a no-op success.
func (c *client) opNudge(name, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "nudge"}, nudgeRequest{Text: text}, nil)
}

// opSetMeta stores session metadata.
func (c *client) opSetMeta(name, key, value string) error {
	return c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "meta", key}, metaRequest{Value: value}, nil)
}

// opGetMeta retrieves session metadata; a missing session yields the empty
// string (the documented not-set sentinel).
func (c *client) opGetMeta(name, key string) (string, error) {
	var out metaResponse
	err := c.do(context.Background(), c.timeout, http.MethodGet, []string{"session", name, "meta", key}, nil, &out)
	if errors.Is(err, errSessionGone) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return out.Value, nil
}

// opRemoveMeta removes session metadata; missing is success.
func (c *client) opRemoveMeta(name, key string) error {
	return c.idempotent(c.do(context.Background(), c.timeout, http.MethodDelete, []string{"session", name, "meta", key}, nil, nil))
}

// opPeek captures recent output from the session.
func (c *client) opPeek(name string, lines int) (string, error) {
	var out peekResponse
	if err := c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "peek"}, peekRequest{Lines: lines}, &out); err != nil {
		return "", err
	}
	return out.Output, nil
}

// opSendKeys sends raw key tokens to the session; missing is success.
func (c *client) opSendKeys(name string, keys []string) error {
	return c.idempotent(c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "keys"}, sendKeysRequest{Keys: keys}, nil))
}

// opClearScrollback truncates the remote output buffer via exec; missing is
// success.
func (c *client) opClearScrollback(name string) error {
	_, err := c.exec(name, "truncate -s0 /workspace/.gc-scrollback 2>/dev/null || true")
	return c.idempotent(err)
}

// exec posts a shell command to the session and decodes the result.
func (c *client) exec(name, cmd string) (execResponse, error) {
	var out execResponse
	err := c.do(context.Background(), c.timeout, http.MethodPost, []string{"session", name, "exec"}, execRequest{Cmd: cmd}, &out)
	return out, err
}

// idempotent collapses a session-gone error to success so stop/remove/key
// ops never fail on an already-absent session.
func (c *client) idempotent(err error) error {
	if errors.Is(err, errSessionGone) {
		return nil
	}
	return err
}

// parseLines is the integer arg parser for peek; it tolerates empty/blank.
func parseLines(arg string) (int, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, nil
	}
	return strconv.Atoi(arg)
}
