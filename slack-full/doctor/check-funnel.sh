#!/bin/sh
set -eu

env_file="${XDG_CONFIG_HOME:-$HOME/.config}/gc-slack-adapter/env"
listen="${LISTEN_PUBLIC:-}"
if [ -z "$listen" ] && [ -f "$env_file" ]; then
  listen="$(sed -n 's/^\(export \)\{0,1\}LISTEN_PUBLIC=//p' "$env_file" | tail -n 1 | tr -d '"'"'"'')"
fi
port="${listen:-:8765}"
port="${port##*:}"

if ! command -v tailscale >/dev/null 2>&1; then
  echo "tailscale not found; skipping Funnel check — public ingress to the adapter's /slack/events listener (:${port}) must be provided another way"
  exit 0
fi

if ! tailscale funnel status 2>/dev/null | grep -q ":${port}\b"; then
  echo "no Tailscale Funnel rule forwarding to adapter port ${port}"
  echo "Re-add it (e.g. tailscale funnel --bg ${port}) — the rule is not declared in the city, so a host reboot or 'tailscale funnel reset' silently stops Slack traffic."
  exit 2
fi

echo "Tailscale Funnel rule to adapter port ${port} present"
