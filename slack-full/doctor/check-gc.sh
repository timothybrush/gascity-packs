#!/bin/sh
set -eu

if ! command -v gc >/dev/null 2>&1; then
  echo "gc CLI not found"
  echo "Install or expose the gc binary so slack-full commands can resolve the city and deliver protocol nudges."
  exit 2
fi

echo "gc CLI available"
