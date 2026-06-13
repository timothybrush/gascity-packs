// Command gc-runtime-cloudflare is a Runtime Provider Protocol (RPP) v0
// executable that proxies Gas City session operations to a Cloudflare
// Worker runtime API. It is the pack-shipped replacement for the gascity
// in-tree internal/runtime/cloudflare provider: gc resolves
// session = "cloudflare" to this executable through a pack [runtimes.<name>]
// declaration, so the runtime evolves and ships independently of the gc
// binary (RPP delivery independence).
//
// Calling convention (no shell wrapping — gc execs the binary directly):
//
//	gc-runtime-cloudflare <op> [<session-name>] [args...]
//
// Inputs that vary per op arrive on stdin (start config JSON, nudge text,
// meta value, process names). Results are written to stdout. Exit codes:
//
//	0  success
//	1  failure (message on stderr)
//	2  unknown / unimplemented op (forward-compatible no-op for the caller)
//
// Configuration comes from the environment:
//
//	GC_CLOUDFLARE_RUNTIME_URL    required: absolute base URL of the Worker API
//	GC_CLOUDFLARE_RUNTIME_TOKEN  optional: bearer token sent to the Worker
package main

import (
	"fmt"
	"io"
	"os"
)

const (
	envEndpoint = "GC_CLOUDFLARE_RUNTIME_URL"
	envToken    = "GC_CLOUDFLARE_RUNTIME_TOKEN"

	exitOK      = 0
	exitError   = 1
	exitUnknown = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run dispatches one RPP operation. It is separated from main so tests can
// drive it with in-memory streams and assert exit codes.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "gc-runtime-cloudflare: missing operation")
		return exitError
	}
	op := args[0]
	rest := args[1:]

	// Ops that need no backend must answer before we require the endpoint,
	// so a misconfigured or unset GC_CLOUDFLARE_RUNTIME_URL never turns a
	// handshake, a constant answer, or an unimplemented-op probe into a
	// spurious failure.
	switch op {
	case "protocol":
		// Must answer before any session (and any backend) exists.
		fmt.Fprintln(stdout, protocolHandshakeJSON)
		return exitOK
	case "is-attached":
		// The Cloudflare runtime has no local TTY; sessions are always
		// detached. report-attachment is deliberately NOT advertised, so
		// gc never trusts this — it is implemented only so a direct probe
		// answers cleanly, and it needs no backend.
		if len(rest) == 0 {
			fmt.Fprintf(stderr, "gc-runtime-cloudflare %s: missing session name\n", op)
			return exitError
		}
		fmt.Fprintln(stdout, boolText(false))
		return exitOK
	}
	if !backendOps[op] {
		// Unknown or deliberately unsupported op (attach, list-running,
		// copy-to, check-image, watch-startup, run-live, …). Exit 2 is the
		// RPP forward-compatibility signal the caller treats as a no-op
		// success — and it must not depend on backend configuration.
		return exitUnknown
	}

	c, err := newClient(os.Getenv(envEndpoint), os.Getenv(envToken))
	if err != nil {
		fmt.Fprintf(stderr, "gc-runtime-cloudflare: %v\n", err)
		return exitError
	}

	fail := func(err error) int {
		fmt.Fprintf(stderr, "gc-runtime-cloudflare %s: %v\n", op, err)
		return exitError
	}
	needName := func() (string, bool) {
		if len(rest) == 0 {
			fmt.Fprintf(stderr, "gc-runtime-cloudflare %s: missing session name\n", op)
			return "", false
		}
		return rest[0], true
	}

	switch op {
	case "start":
		name, ok := needName()
		if !ok {
			return exitError
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fail(fmt.Errorf("reading start config: %w", err))
		}
		if err := c.opStart(name, data); err != nil {
			return fail(err)
		}
		return exitOK

	case "stop":
		name, ok := needName()
		if !ok {
			return exitError
		}
		if err := c.opStop(name); err != nil {
			return fail(err)
		}
		return exitOK

	case "interrupt":
		name, ok := needName()
		if !ok {
			return exitError
		}
		if err := c.opInterrupt(name); err != nil {
			return fail(err)
		}
		return exitOK

	case "is-running":
		name, ok := needName()
		if !ok {
			return exitError
		}
		fmt.Fprintln(stdout, boolText(c.opIsRunning(name)))
		return exitOK

	case "get-last-activity":
		name, ok := needName()
		if !ok {
			return exitError
		}
		t, err := c.opGetLastActivity(name)
		if err != nil {
			return fail(err)
		}
		if !t.IsZero() {
			fmt.Fprintln(stdout, t.UTC().Format(timeRFC3339Nano))
		}
		return exitOK

	case "process-alive":
		name, ok := needName()
		if !ok {
			return exitError
		}
		names, err := readLines(stdin)
		if err != nil {
			return fail(fmt.Errorf("reading process names: %w", err))
		}
		fmt.Fprintln(stdout, boolText(c.opProcessAlive(name, names)))
		return exitOK

	case "nudge":
		name, ok := needName()
		if !ok {
			return exitError
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fail(fmt.Errorf("reading nudge text: %w", err))
		}
		if err := c.opNudge(name, string(data)); err != nil {
			return fail(err)
		}
		return exitOK

	case "set-meta":
		name, key, ok := needNameKey(rest, op, stderr)
		if !ok {
			return exitError
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fail(fmt.Errorf("reading meta value: %w", err))
		}
		if err := c.opSetMeta(name, key, string(data)); err != nil {
			return fail(err)
		}
		return exitOK

	case "get-meta":
		name, key, ok := needNameKey(rest, op, stderr)
		if !ok {
			return exitError
		}
		value, err := c.opGetMeta(name, key)
		if err != nil {
			return fail(err)
		}
		fmt.Fprint(stdout, value)
		return exitOK

	case "remove-meta":
		name, key, ok := needNameKey(rest, op, stderr)
		if !ok {
			return exitError
		}
		if err := c.opRemoveMeta(name, key); err != nil {
			return fail(err)
		}
		return exitOK

	case "peek":
		name, ok := needName()
		if !ok {
			return exitError
		}
		lines := 0
		if len(rest) > 1 {
			n, err := parseLines(rest[1])
			if err != nil {
				return fail(fmt.Errorf("invalid line count %q: %w", rest[1], err))
			}
			lines = n
		}
		out, err := c.opPeek(name, lines)
		if err != nil {
			return fail(err)
		}
		fmt.Fprint(stdout, out)
		return exitOK

	case "send-keys":
		name, ok := needName()
		if !ok {
			return exitError
		}
		if err := c.opSendKeys(name, rest[1:]); err != nil {
			return fail(err)
		}
		return exitOK

	case "clear-scrollback":
		name, ok := needName()
		if !ok {
			return exitError
		}
		if err := c.opClearScrollback(name); err != nil {
			return fail(err)
		}
		return exitOK

	default:
		// Unreachable: backendOps gates entry to this switch. Kept as a
		// defensive exit-2 so an op added to backendOps without a case here
		// degrades to the forward-compatible no-op rather than panicking.
		return exitUnknown
	}
}

// backendOps is the set of RPP operations gc-runtime-cloudflare implements
// against the Worker. Ops outside it (and outside the no-backend cases
// handled before the registry endpoint is required) exit 2 — the RPP
// forward-compatibility signal — without touching the backend.
var backendOps = map[string]bool{
	"start":             true,
	"stop":              true,
	"interrupt":         true,
	"is-running":        true,
	"get-last-activity": true,
	"process-alive":     true,
	"nudge":             true,
	"set-meta":          true,
	"get-meta":          true,
	"remove-meta":       true,
	"peek":              true,
	"send-keys":         true,
	"clear-scrollback":  true,
}

func needNameKey(rest []string, op string, stderr io.Writer) (name, key string, ok bool) {
	if len(rest) < 2 {
		fmt.Fprintf(stderr, "gc-runtime-cloudflare %s: requires <session-name> <key>\n", op)
		return "", "", false
	}
	return rest[0], rest[1], true
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
