#!/usr/bin/env bash
# Install the gc-runtime-cloudflare executable onto PATH via `go install`.
# This is the pack's install step (Gas City RUNTIME-SEL-011): the
# pack-runtimes doctor check then verifies the binary is installed and
# answers the RPP protocol handshake.
#
# The version is pinned to the pack version so bumping [pack].version and
# re-installing is the delivery-independence lever — a new runtime binary
# under an unchanged gc binary.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Build from the vendored source in this pack so an install works offline
# and from a checkout. (To install a published, pinned version instead:
#   go install github.com/gastownhall/gc-runtime-cloudflare@v0.1.0
# once the module is tagged in its own repository.)
echo "runtime-cloudflare: installing gc-runtime-cloudflare via go install"
( cd "$here/runtime" && go install . )

if ! command -v gc-runtime-cloudflare >/dev/null 2>&1; then
  echo "runtime-cloudflare: installed, but gc-runtime-cloudflare is not on PATH." >&2
  echo "Add \$(go env GOBIN) or \$(go env GOPATH)/bin to PATH." >&2
  exit 1
fi
echo "runtime-cloudflare: installed $(command -v gc-runtime-cloudflare)"
