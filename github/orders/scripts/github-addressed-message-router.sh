#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
order_dir="${ORDER_DIR:-$(cd "$script_dir/.." && pwd -P)}"
pack_root="${PACK_DIR:-$(cd "$order_dir/.." && pwd -P)}"
sweep_limit="${GITHUB_INTAKE_ADDRESS_SWEEP_LIMIT:-50}"
sweep_days="${GITHUB_INTAKE_ADDRESS_SWEEP_DAYS:-7}"

rc=0
python3 "$pack_root/scripts/github_intake_addressed.py" sweep-comments --limit "$sweep_limit" --days "$sweep_days" --fail-on-error || rc=$?
python3 "$pack_root/scripts/github_intake_addressed.py" router-scan --fail-on-error || rc=$?
exit "$rc"
