#!/usr/bin/env bash
# Build gc-runtime-cloudflare and its in-memory fake Worker, then run the
# Gas City RPP conformance suite against the binary with no live Cloudflare
# account or network. This is the pack's CI gate (RUNTIME-RPP-010): a green
# run proves the pack-shipped runtime satisfies the Runtime Provider
# Protocol independently of the gascity codebase.
#
# Requires `gc` on PATH (install with a pinned:
#   go install github.com/gastownhall/gascity/cmd/gc@<pin>
# — a tool install, not a source import, so the pack keeps zero gascity Go
# dependencies). Override the binary with GC_BIN=/path/to/gc.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
runtime_dir="$here/runtime"
bindir="$(mktemp -d)"
trap 'rm -rf "$bindir"; [ -n "${worker_pid:-}" ] && kill "$worker_pid" 2>/dev/null || true' EXIT

gc_bin="${GC_BIN:-gc}"
if ! command -v "$gc_bin" >/dev/null 2>&1 && [ ! -x "$gc_bin" ]; then
  echo "conformance: gc binary not found (set GC_BIN or install gc)" >&2
  exit 1
fi

echo "conformance: building gc-runtime-cloudflare + fakeworker"
( cd "$runtime_dir" && go build -o "$bindir/gc-runtime-cloudflare" . )
( cd "$runtime_dir" && go build -o "$bindir/fakeworker" ./fakeworker )

echo "conformance: starting fake Worker"
worker_out="$(mktemp)"
"$bindir/fakeworker" -addr 127.0.0.1:0 >"$worker_out" 2>/dev/null &
worker_pid=$!
# Wait for the worker to announce its base URL on the first stdout line.
for _ in $(seq 1 50); do
  url="$(head -n1 "$worker_out" 2>/dev/null || true)"
  [ -n "$url" ] && break
  sleep 0.1
done
if [ -z "${url:-}" ]; then
  echo "conformance: fake Worker did not report a URL" >&2
  exit 1
fi
echo "conformance: fake Worker at $url"

export GC_CLOUDFLARE_RUNTIME_URL="$url"
export GC_CLOUDFLARE_RUNTIME_TOKEN=""

echo "conformance: gc runtime check"
"$gc_bin" runtime check "$bindir/gc-runtime-cloudflare"
