#!/bin/sh
# gc <binding> claim — atomically claim and verify one routed work bead.

set -eu

if [ -z "${GC_PACK_DIR:-}" ]; then
    echo "gc gascity claim: missing Gas City pack context" >&2
    exit 1
fi

while [ "$#" -gt 0 ]; do
    case "$1" in
        gc|gascity|claim|--city=*|--rig=*)
            shift
            ;;
        --city|--rig)
            if [ "$#" -lt 2 ]; then
                echo "gc ${GC_PACK_NAME:-gascity} claim: missing value for $1" >&2
                exit 2
            fi
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    cat "$GC_PACK_DIR/commands/claim/help.md"
    exit 0
fi

if [ "$#" -ne 0 ]; then
    echo "gc ${GC_PACK_NAME:-gascity} claim: no arguments accepted" >&2
    exit 2
fi

if ! command -v gc >/dev/null 2>&1; then
    echo "gc ${GC_PACK_NAME:-gascity} claim: gc binary not in PATH" >&2
    exit 1
fi

if ! command -v bd >/dev/null 2>&1; then
    echo "gc ${GC_PACK_NAME:-gascity} claim: bd binary not in PATH" >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "gc ${GC_PACK_NAME:-gascity} claim: python3 not in PATH" >&2
    exit 1
fi

json_pick() {
    python3 -c '
import json
import sys

path = sys.argv[1]
try:
    data = json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)

if isinstance(data, list):
    data = data[0] if data else {}
if not isinstance(data, dict):
    print("")
    raise SystemExit(0)

if path.startswith("metadata:"):
    metadata = data.get("metadata") or {}
    value = metadata.get(path.split(":", 1)[1], "") if isinstance(metadata, dict) else ""
else:
    value = data.get(path, "")

if value is None:
    value = ""
print(value if isinstance(value, str) else str(value))
' "$1"
}

EXPECTED_ASSIGNEE="${BEADS_ACTOR:-${GC_SESSION_NAME:-${GC_SESSION_ID:-${GC_AGENT:-}}}}"
EXPECTED_ROUTE="${GC_TEMPLATE:-${GC_AGENT:-}}"

if [ -z "$EXPECTED_ASSIGNEE" ]; then
    echo "gc ${GC_PACK_NAME:-gascity} claim: missing expected assignee" >&2
    exit 1
fi

claim_file="$(mktemp)"
show_file="$(mktemp)"
err_file="$(mktemp)"
trap 'rm -f "$claim_file" "$show_file" "$err_file"' EXIT HUP INT TERM

while :; do
    if gc hook --claim --drain-ack --json >"$claim_file" 2>"$err_file"; then
        claim_code=0
    else
        claim_code=$?
    fi

    claim_action="$(json_pick action <"$claim_file")"
    work_id="$(json_pick bead_id <"$claim_file")"
    claim_assignee="$(json_pick assignee <"$claim_file")"
    claim_route="$(json_pick route <"$claim_file")"

    if [ "$claim_action" = "drain" ]; then
        cat "$claim_file"
        exit 0
    fi

    if [ "$claim_code" -ne 0 ] || [ "$claim_action" != "work" ] || [ -z "$work_id" ]; then
        if [ -s "$err_file" ]; then
            printf 'CLAIM_REJECTED gc hook --claim failed: %s\n' "$(sed -n '1p' "$err_file")" >&2
        else
            echo "CLAIM_REJECTED unexpected gc hook --claim result" >&2
        fi
        sleep 2
        continue
    fi

    if ! bd show "$work_id" --json >"$show_file" 2>"$err_file"; then
        if [ -s "$err_file" ]; then
            printf 'CLAIM_REJECTED bead read failed for %s: %s\n' "$work_id" "$(sed -n '1p' "$err_file")" >&2
        else
            printf 'CLAIM_REJECTED bead read failed for %s\n' "$work_id" >&2
        fi
        sleep 2
        continue
    fi

    claim_id="$(json_pick id <"$show_file")"
    claim_status="$(json_pick status <"$show_file")"
    show_assignee="$(json_pick assignee <"$show_file")"
    show_route="$(json_pick metadata:gc.routed_to <"$show_file")"
    claim_root="$(json_pick metadata:gc.root_bead_id <"$show_file")"
    claim_group="$(json_pick metadata:gc.continuation_group <"$show_file")"
    [ -n "$show_assignee" ] && claim_assignee="$show_assignee"
    [ -n "$show_route" ] && claim_route="$show_route"

    if [ "$claim_id" != "$work_id" ]; then
        printf 'CLAIM_REJECTED verification failed for %s\n' "$work_id" >&2
    elif [ "$claim_status" != "open" ] && [ "$claim_status" != "in_progress" ]; then
        printf 'CLAIM_REJECTED unexpected status for %s: %s\n' "$work_id" "$claim_status" >&2
    elif [ "$claim_assignee" != "$EXPECTED_ASSIGNEE" ]; then
        printf 'CLAIM_REJECTED assignee mismatch for %s\n' "$work_id" >&2
    elif [ -n "$EXPECTED_ROUTE" ] && [ -n "$claim_route" ] && [ "$claim_route" != "$EXPECTED_ROUTE" ]; then
        printf 'CLAIM_REJECTED route mismatch for %s\n' "$work_id" >&2
    else
        break
    fi

    sleep 2
done

python3 - "$claim_file" "$show_file" <<'PY'
import json
import sys

claim = json.load(open(sys.argv[1], encoding="utf-8"))
bead = json.load(open(sys.argv[2], encoding="utf-8"))
if isinstance(claim, list):
    claim = claim[0] if claim else {}
if isinstance(bead, list):
    bead = bead[0] if bead else {}
metadata = bead.get("metadata") or {}
print(json.dumps({
    "action": "work",
    "bead_id": bead["id"],
    "root_bead_id": metadata.get("gc.root_bead_id", ""),
    "continuation_group": metadata.get("gc.continuation_group", ""),
    "bead": bead,
}, separators=(",", ":")))
PY
