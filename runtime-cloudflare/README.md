# runtime-cloudflare

A Gas City **runtime pack**: it ships a runtime *executable*, not a service.
The executable `gc-runtime-cloudflare` speaks the Runtime Provider Protocol
(RPP v0) and proxies Gas City session operations to a Cloudflare Worker
runtime API. It is the pack-delivered replacement for the former in-tree
`internal/runtime/cloudflare` provider.

The point of the pack is **delivery independence**: the Cloudflare runtime
evolves and ships on its own cadence, with zero Go dependencies on the
gascity codebase, and is selected by configuration alone.

## Layout

```
runtime-cloudflare/
├── pack.toml              # declares [runtimes.cloudflare] → gc-runtime-cloudflare
├── install.sh            # go install the executable onto PATH
├── conformance.sh        # gc runtime check against an in-memory fake Worker
└── runtime/              # nested Go module (zero gascity imports)
    ├── go.mod            # module github.com/gastownhall/gc-runtime-cloudflare
    ├── main.go           # RPP argv dispatch
    ├── client.go         # Cloudflare Worker HTTP client
    ├── ops.go            # one RPP op → Worker call(s)
    ├── protocol.go       # protocol handshake (version 0, report-activity)
    └── fakeworker/       # in-memory Worker for offline conformance + CI
```

## Use

```toml
# city.toml
[imports.runtime-cloudflare]
source = "<path-or-registry>/runtime-cloudflare"

[session]
provider = "cloudflare"
```

```bash
./install.sh                 # put gc-runtime-cloudflare on PATH
gc doctor                    # the pack-runtimes check verifies install + handshake
```

Runtime configuration is read from the environment by the executable:

| Variable | Required | Meaning |
|----------|----------|---------|
| `GC_CLOUDFLARE_RUNTIME_URL` | yes | absolute base URL of the Worker runtime API |
| `GC_CLOUDFLARE_RUNTIME_TOKEN` | no | bearer token sent to the Worker |

## Conformance

```bash
GC_BIN=$(command -v gc) ./conformance.sh
```

`conformance.sh` builds the executable and an in-memory fake Worker, then
runs `gc runtime check` through the full RPP lifecycle round-trip — no live
Cloudflare account or network required. This is the pack's CI gate
(`gc` itself is a tool dependency installed with a pinned
`go install github.com/gastownhall/gascity/cmd/gc@<pin>`, never imported, so
the pack keeps zero gascity Go dependencies).

## RPP operations

| Op | Backing Worker call | Notes |
|----|---------------------|-------|
| `protocol` | none | `{"version":0,"capabilities":["report-activity"]}` |
| `start <name>` | `POST /session` | start config JSON on stdin, forwarded verbatim |
| `stop <name>` | `POST /session/:name/stop` | idempotent (404 → ok) |
| `is-running <name>` | `GET /session/:name/status` | `.alive` |
| `interrupt <name>` | `exec pkill -INT -u $(id -u)` | best-effort, idempotent |
| `get-last-activity <name>` | `GET /session/:name/status` | `.record.createdAt` (RFC3339) |
| `process-alive <name>` | `exec pgrep -Ef` | process names on stdin, 1/line |
| `nudge <name>` | `POST /session/:name/nudge` | text on stdin |
| `set/get/remove-meta <name> <key>` | `…/meta/:key` | value on stdin for set |
| `peek <name> <lines>` | `POST /session/:name/peek` | captured output to stdout |
| `send-keys <name> <keys…>` | `POST /session/:name/keys` | idempotent |
| `clear-scrollback <name>` | `exec truncate` | idempotent |

`is-attached` returns `false` (no local TTY; `report-attachment` is not
advertised, so gc never trusts it). `attach`, `list-running`, `copy-to`,
`check-image`, `watch-startup`, and any future op exit 2 — the RPP
forward-compatibility signal that the caller treats as a no-op success.

## Delivery-independence demo

This is the PoC exit criterion (`ga-6qwfkb`, `RUNTIME-PLAN-004`): change the
runtime's behavior under an **unchanged `gc` binary** by bumping the pack.

1. Select the pack runtime and confirm a baseline:
   ```bash
   ./install.sh
   gc runtime check "$(command -v gc-runtime-cloudflare)"   # against a live or fake Worker
   ```
2. Change the runtime (e.g. a new Worker endpoint shape, an added
   capability, a bug fix) under `runtime/`, bump `[pack].version` and the
   `install.sh` pin, and re-install:
   ```bash
   ./install.sh
   ```
3. The same `gc` binary now drives the new runtime behavior — no gascity
   rebuild, no gascity release. The only thing that changed is the pack.

## t3bridge extraction checklist

The PoC's second exit criterion is a written assessment of extracting the
`t3bridge` runtime to a pack the same way. Captured in
[`docs/t3bridge-extraction-checklist.md`](docs/t3bridge-extraction-checklist.md).
