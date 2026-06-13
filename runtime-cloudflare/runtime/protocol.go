package main

import (
	"bufio"
	"io"
	"strings"
	"time"
)

// timeRFC3339Nano is the timestamp format gc parses for get-last-activity.
const timeRFC3339Nano = time.RFC3339Nano

// protocolHandshakeJSON is the response to the `protocol` op. The Cloudflare
// runtime backs get-last-activity from the session status record, so it
// advertises report-activity. It has no local TTY, so report-attachment is
// deliberately omitted — gc then never trusts is-attached and treats every
// session as detached.
const protocolHandshakeJSON = `{"version":0,"capabilities":["report-activity"]}`

// readLines reads stdin into trimmed, non-empty lines (the process-alive
// input convention: one process name per line).
func readLines(r io.Reader) ([]string, error) {
	var out []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			out = append(out, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
