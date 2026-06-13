package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout      = 30 * time.Second
	defaultStartTimeout = 120 * time.Second
	maxResponseBytes    = 1 << 20
)

// errSessionGone marks a Worker response that means the session does not
// exist (HTTP 404). Lifecycle ops treat it as already-stopped so stop and
// metadata removal stay idempotent — the same contract the in-tree
// cloudflare provider enforced via runtime.IsSessionGone.
var errSessionGone = errors.New("cloudflare runtime: session gone")

// errSessionExists marks an HTTP 409 from the Worker (start collision).
var errSessionExists = errors.New("cloudflare runtime: session already exists")

// client speaks the Cloudflare Worker runtime HTTP API. It is the RPP
// proxy's only dependency on the remote runtime: every Runtime Provider
// Protocol operation maps to one or more Worker calls. Stdlib-only — the
// pack ships with zero gascity imports so it can evolve and deploy
// independently of the gc binary (RPP delivery independence).
type client struct {
	endpoint     *url.URL
	token        string
	timeout      time.Duration
	startTimeout time.Duration
	http         *http.Client
}

// newClient builds a Worker client from the runtime endpoint and optional
// bearer token. The endpoint must be an absolute URL.
func newClient(endpoint, token string) (*client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("cloudflare runtime endpoint is required (set %s)", envEndpoint)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing cloudflare runtime endpoint: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("cloudflare runtime endpoint must be an absolute URL, got %q", endpoint)
	}
	return &client{
		endpoint:     parsed,
		token:        token,
		timeout:      defaultTimeout,
		startTimeout: defaultStartTimeout,
		http:         &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (c *client) do(ctx context.Context, timeout time.Duration, method string, parts []string, body, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling cloudflare runtime request: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	target := c.urlFor(parts...).String()
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return fmt.Errorf("building cloudflare runtime request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare runtime request: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("reading cloudflare runtime response: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing cloudflare runtime response: %w", closeErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp.StatusCode, target, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding cloudflare runtime response: %w", err)
	}
	return nil
}

func (c *client) urlFor(parts ...string) *url.URL {
	u := *c.endpoint
	base := strings.TrimRight(u.Path, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	if len(escaped) > 0 {
		u.Path = base + "/" + strings.Join(escaped, "/")
	} else {
		u.Path = base
	}
	return &u
}

func statusError(status int, target string, data []byte) error {
	msg := statusText(status, data)
	switch status {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s: %s", errSessionGone, target, msg)
	case http.StatusConflict:
		return fmt.Errorf("%w: %s: %s", errSessionExists, target, msg)
	default:
		return fmt.Errorf("cloudflare runtime %s: status %d: %s", target, status, msg)
	}
}

func statusText(status int, data []byte) string {
	var payload errorResponse
	if err := json.Unmarshal(data, &payload); err == nil {
		switch {
		case payload.Error != "":
			return payload.Error
		case payload.Message != "":
			return payload.Message
		}
	}
	if text := strings.TrimSpace(string(data)); text != "" {
		return text
	}
	return http.StatusText(status)
}

// --- Worker wire types (mirror the in-tree provider's Worker contract) ---

type errorResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// startRequest is the body for POST /session. config is forwarded verbatim
// from the RPP start-config JSON on stdin so the Worker keeps receiving the
// exact payload shape the in-tree provider sent.
type startRequest struct {
	SessionID string          `json:"sessionId"`
	Config    json.RawMessage `json:"config,omitempty"`
}

type execRequest struct {
	Cmd string `json:"cmd"`
}

type execResponse struct {
	ExitCode int  `json:"exitCode"`
	Success  bool `json:"success"`
}

type sessionStatusResponse struct {
	Alive  bool `json:"alive"`
	Record struct {
		CreatedAt string `json:"createdAt"`
	} `json:"record"`
}

type nudgeRequest struct {
	Text string `json:"text"`
}

type metaRequest struct {
	Value string `json:"value"`
}

type metaResponse struct {
	Value string `json:"value"`
}

type peekRequest struct {
	Lines int `json:"lines,omitempty"`
}

type peekResponse struct {
	Output string `json:"output"`
}

type sendKeysRequest struct {
	Keys []string `json:"keys,omitempty"`
}

// shellQuoteSingle wraps s in single quotes, escaping embedded single
// quotes, for safe interpolation into a remote shell command.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
