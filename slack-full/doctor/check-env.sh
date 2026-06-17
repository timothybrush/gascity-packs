#!/bin/sh
set -eu

env_file="${XDG_CONFIG_HOME:-$HOME/.config}/gc-slack-adapter/env"

missing=""
for var in SLACK_WORKSPACE_ID SLACK_BOT_TOKEN SLACK_SIGNING_SECRET GC_CITY_NAME; do
  if eval "[ -n \"\${$var:-}\" ]"; then
    continue
  fi
  if [ -f "$env_file" ] && grep -q "^\(export \)\{0,1\}${var}=." "$env_file"; then
    continue
  fi
  missing="$missing $var"
done

if [ -n "$missing" ]; then
  echo "missing adapter env vars:$missing"
  echo "Export them or add them to $env_file (mode 0600) so the adapter's supervisor inherits them — the adapter Fatals at start without them."
  exit 2
fi

echo "adapter env vars present"
