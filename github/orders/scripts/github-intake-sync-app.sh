#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
order_dir="${ORDER_DIR:-$(cd "$script_dir/.." && pwd -P)}"
pack_root="${PACK_DIR:-$(cd "$order_dir/.." && pwd -P)}"

python3 "$pack_root/scripts/github_intake_sync_app.py" --quiet
