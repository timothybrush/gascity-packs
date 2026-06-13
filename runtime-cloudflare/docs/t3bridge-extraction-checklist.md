# t3bridge → runtime pack: extraction checklist

PoC exit criterion for `ga-6qwfkb` / `RUNTIME-PLAN-004`. `runtime-cloudflare`
proved a **stateless HTTP-proxy** runtime extracts cleanly to an RPP pack.
`t3bridge` is the harder case and the real test of whether RPP is enough to
move *every* runtime out of the gascity binary. This is a written
assessment, not an implementation.

## What t3bridge does today

`internal/runtime/t3bridge` is a JSON-RPC bridge to the T3 Code app. Unlike
the Cloudflare provider (a thin HTTP client), it is **stateful and
gascity-coupled**:

- Imports `internal/beads` and `internal/events` directly — it reads/writes
  the work ledger and publishes to the event bus from inside the provider.
- Runs a per-session **event-watcher goroutine** (`runEventWatcher` /
  `ensureEventWatcher`) that streams T3 thread activity and projects it.
- Manages **git worktrees** for threads (`rpcCreateWorktree`,
  `rpcRemoveWorktree`, `removeWorktreeForThread`).
- Maintains **assignment/thread-metadata projection** into beads
  (`refreshAssignmentProjection`, `waitForThreadGCMetadata`,
  `dispatchThreadMeta`).
- Caches RPC snapshots and tracks recent-start debouncing in memory.

## RPP operations the pack executable must speak

The mechanical session surface maps to RPP directly (same ops
`gc-runtime-cloudflare` implements):

| RPP op | t3bridge method | Notes |
|--------|-----------------|-------|
| `start <name>` | `Start` | creates/binds a T3 thread; runs pre-start worktree setup |
| `stop <name>` | `Stop` → `dispatchThreadSessionStop` | |
| `is-running <name>` | `IsRunning` → `threadSessionStatus` | |
| `interrupt <name>` | `Interrupt` → `dispatchTurnInterrupt` | |
| `list-running <prefix>` | `ListRunning` | t3bridge **implements** this (cloudflare did not) |
| `nudge <name>` | `dispatchTurnStart` | turn text on stdin |
| `process-alive` / `is-attached` | `ProcessAlive` / `IsAttached` | both effectively constant |

The hard part is **not** these ops. It is everything the in-process
provider does *besides* answering ops.

## Blockers an RPP executable must resolve

1. **Ledger access without `internal/beads`.** A pack binary cannot import
   gascity. Today the provider reads/writes beads in-process. Options:
   - **bd CLI** — shell out to `bd` for ledger reads/writes (thread-meta
     projection, assignment). Available wherever the agent runs; matches
     the "if a tool has a CLI, the agent uses it" principle. Highest
     fidelity, no new surface.
   - **gc API** — call the HTTP control plane for ledger ops. Needs the API
     reachable from the runtime host and an auth token.
   - Verdict: **bd CLI for writes/reads**, gc API only if the runtime runs
     somewhere `bd` cannot reach the Dolt store.

2. **Event publication without `internal/events`.** The provider publishes
   activity/state-change events. RPP has no "emit event" op. Options:
   - Let the gc side own event emission: gc already records lifecycle
     events around the ops it drives, so the pack may not need to emit at
     all — confirm which `recordStateChange`/`dispatchActivity` events are
     load-bearing vs. observable by gc from op results.
   - If genuinely pack-originated, emit via gc API (`POST` to the event
     endpoint) or via a `bd`-backed message bead.
   - Verdict: **audit each emit**; push as many as possible to the gc side;
     remainder via gc API.

3. **The event-watcher goroutine.** `runEventWatcher` is a long-lived stream
   consumer per session. RPP ops are short-lived execs — there is no
   persistent provider process. Options:
   - A `watch-startup`-style streaming op (gc already has the
     `watch-startup` op shape: stdout one JSON event per line until EOF).
     Extend that pattern to a general activity stream the executable serves
     for the session's lifetime.
   - Or host the watcher in the T3 Code app itself (see #5) and have it
     write directly to the ledger via `bd`.
   - Verdict: **prefer T3-Code-hosted watcher**; the pack binary stays
     request/response.

4. **Worktree management.** `rpcCreateWorktree`/`rpcRemoveWorktree` run on
   the controller host. The RPP `start` config already carries `pre_start`
   (worktree setup) and the provider can run worktree teardown on `stop`.
   No gascity import needed — plain git. **Low risk.**

5. **Who hosts the executable.** Cloudflare's binary is self-contained (HTTP
   client). t3bridge's natural host is **T3 Code** itself:
   - T3 Code ships `gc-runtime-t3bridge` (or exposes the RPC and a thin
     shim does), keeping the JSON-RPC client next to the app it talks to.
   - The pack then declares `[runtimes.t3bridge] command = "gc-runtime-t3bridge"`
     and T3 Code's installer puts it on PATH — same shape as this pack.
   - Verdict: **T3 Code hosts and ships the executable**; gascity-packs
     carries only the pack.toml + conformance harness + a fake T3 RPC
     endpoint for CI.

## Recommended sequence

1. Land a `watch-startup`/activity-stream RPP op (or confirm gc-side events
   suffice) — unblocks #3.
2. Port the mechanical ops to a `gc-runtime-t3bridge` binary using **bd CLI**
   for ledger access and **plain git** for worktrees (#1, #4).
3. Decide event ownership per-emit (#2); move watcher to T3 Code (#5).
4. Conformance: a fake T3 RPC server (mirroring this pack's `fakeworker`)
   so `gc runtime check` is green in CI with no live T3 Code.
5. Delete `internal/runtime/t3bridge`; `session = "t3bridge"` resolves via
   the pack.

## Bottom line

RPP's op surface is **sufficient** for t3bridge's session lifecycle. The
real work is severing the two in-process couplings RPP does not cover —
**ledger access** (→ bd CLI) and **event publication** (→ gc-side or gc
API) — plus relocating the **streaming watcher** to T3 Code. None require a
new primitive; they require routing those side effects through existing
CLI/API surfaces instead of Go imports. Cloudflare was extractable as-is;
t3bridge is extractable after the ledger/event/watcher rerouting above.
