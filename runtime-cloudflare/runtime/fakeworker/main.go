// Command fakeworker is an in-memory stand-in for the Cloudflare Worker
// runtime API. It exists so `gc runtime check` (and the pack's CI
// conformance step) can exercise the full RPP lifecycle round-trip of
// gc-runtime-cloudflare without a live Cloudflare account or network: start
// a session, observe is-running flip true→false across stop, round-trip
// metadata, peek, exec. It implements exactly the endpoints client.go calls.
//
// Usage:
//
//	fakeworker [-addr 127.0.0.1:0]
//
// It prints the chosen base URL (scheme+host) as the first stdout line so a
// harness can capture it, then serves until killed. Point the runtime at it
// with GC_CLOUDFLARE_RUNTIME_URL=<printed URL>.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type session struct {
	createdAt time.Time
	meta      map[string]string
}

type worker struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fakeworker: listen: %v\n", err)
		os.Exit(1)
	}
	w := &worker{sessions: map[string]*session{}}
	// Announce the base URL on the first line so a harness can capture it.
	fmt.Printf("http://%s\n", ln.Addr().String())
	if err := http.Serve(ln, w); err != nil {
		fmt.Fprintf(os.Stderr, "fakeworker: serve: %v\n", err)
		os.Exit(1)
	}
}

func (w *worker) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	// POST /session
	if r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "session" {
		w.start(rw, r)
		return
	}
	if len(parts) >= 2 && parts[0] == "session" {
		name := parts[1]
		switch {
		case len(parts) == 3 && parts[2] == "stop" && r.Method == http.MethodPost:
			w.stop(rw, name)
			return
		case len(parts) == 3 && parts[2] == "status" && r.Method == http.MethodGet:
			w.status(rw, name)
			return
		case len(parts) == 3 && parts[2] == "exec" && r.Method == http.MethodPost:
			w.exec(rw, name)
			return
		case len(parts) == 3 && parts[2] == "nudge" && r.Method == http.MethodPost:
			w.requireSession(rw, name, func() { rw.WriteHeader(http.StatusNoContent) })
			return
		case len(parts) == 3 && parts[2] == "peek" && r.Method == http.MethodPost:
			w.peek(rw, name)
			return
		case len(parts) == 3 && parts[2] == "keys" && r.Method == http.MethodPost:
			w.requireSession(rw, name, func() { rw.WriteHeader(http.StatusNoContent) })
			return
		case len(parts) == 4 && parts[2] == "meta":
			w.meta(rw, r, name, parts[3])
			return
		}
	}
	http.Error(rw, `{"error":"not found"}`, http.StatusNotFound)
}

func (w *worker) start(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.SessionID == "" {
		http.Error(rw, `{"error":"missing sessionId"}`, http.StatusBadRequest)
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.sessions[req.SessionID]; ok {
		http.Error(rw, `{"error":"already exists"}`, http.StatusConflict)
		return
	}
	w.sessions[req.SessionID] = &session{createdAt: time.Now().UTC(), meta: map[string]string{}}
	rw.WriteHeader(http.StatusNoContent)
}

func (w *worker) stop(rw http.ResponseWriter, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.sessions[name]; !ok {
		http.Error(rw, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	delete(w.sessions, name)
	rw.WriteHeader(http.StatusNoContent)
}

func (w *worker) status(rw http.ResponseWriter, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.sessions[name]
	if !ok {
		writeJSON(rw, http.StatusOK, map[string]any{"alive": false})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{
		"alive":  true,
		"record": map[string]any{"createdAt": s.createdAt.Format(time.RFC3339Nano)},
	})
}

func (w *worker) exec(rw http.ResponseWriter, name string) {
	w.mu.Lock()
	_, ok := w.sessions[name]
	w.mu.Unlock()
	if !ok {
		http.Error(rw, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	// A live session: report a clean exit so process-alive reads true.
	writeJSON(rw, http.StatusOK, map[string]any{"exitCode": 0, "success": true})
}

func (w *worker) peek(rw http.ResponseWriter, name string) {
	w.requireSession(rw, name, func() {
		writeJSON(rw, http.StatusOK, map[string]any{"output": ""})
	})
}

func (w *worker) meta(rw http.ResponseWriter, r *http.Request, name, key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.sessions[name]
	if !ok {
		http.Error(rw, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Value string `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		s.meta[key] = req.Value
		rw.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		writeJSON(rw, http.StatusOK, map[string]any{"value": s.meta[key]})
	case http.MethodDelete:
		delete(s.meta, key)
		rw.WriteHeader(http.StatusNoContent)
	default:
		http.Error(rw, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (w *worker) requireSession(rw http.ResponseWriter, name string, ok func()) {
	w.mu.Lock()
	_, exists := w.sessions[name]
	w.mu.Unlock()
	if !exists {
		http.Error(rw, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	ok()
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(body)
}

func splitPath(p string) []string {
	var out []string
	for _, part := range strings.Split(strings.Trim(p, "/"), "/") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
