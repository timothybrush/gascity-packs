#!/usr/bin/env python3
"""Resolve github-intake GitHub App identities from local JSON files."""

from __future__ import annotations

import json
import os
import pathlib
import re
import sys
from typing import Any

SCHEMA_VERSION = "github-intake.github-app-identity.v1"
IDENTITY_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$")


def identity_dir() -> pathlib.Path:
    return pathlib.Path(os.environ.get("GITHUB_INTAKE_IDENTITY_DIR", "config/github-intake/identities")).expanduser()


def validate_identity(identity: str) -> str:
    identity = identity.strip()
    if not IDENTITY_PATTERN.fullmatch(identity):
        raise ValueError("identity must match [A-Za-z0-9][A-Za-z0-9._:-]{0,127}")
    return identity


def load_identity(identity: str) -> dict[str, Any]:
    identity = validate_identity(identity)
    path = identity_dir() / f"{identity}.json"
    with open(path, "r", encoding="utf-8") as handle:
        payload = json.load(handle)
    if not isinstance(payload, dict):
        raise ValueError(f"{path} must contain one JSON object")
    if payload.get("schema_version") != SCHEMA_VERSION:
        raise ValueError(f"{path} must set schema_version to {SCHEMA_VERSION!r}")
    for key in ("app_id", "private_key_pem"):
        if not str(payload.get(key, "")).strip():
            raise ValueError(f"{path} is missing required field {key!r}")
    return payload


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        print("usage: file_identity_resolver.py <identity>", file=sys.stderr)
        return 2
    try:
        payload = load_identity(argv[0])
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"github-intake identity resolution failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(payload, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
