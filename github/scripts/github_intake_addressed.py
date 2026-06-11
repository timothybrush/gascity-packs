#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import urllib.parse
from datetime import datetime, timedelta, timezone
from typing import Any

import github_intake_common as common
import github_intake_service as service


class AddressedError(RuntimeError):
    pass


def gh_timeout_seconds() -> int:
    raw = os.environ.get("GITHUB_INTAKE_GH_TIMEOUT_SECONDS", "30").strip()
    try:
        timeout = int(raw)
    except ValueError:
        timeout = 30
    return max(timeout, 1)


def json_out(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, indent=2, sort_keys=True))


def load_json_command(command: list[str], env: dict[str, str] | None = None) -> Any:
    timeout = gh_timeout_seconds()
    try:
        result = subprocess.run(command, capture_output=True, text=True, check=False, env=env, timeout=timeout)
    except subprocess.TimeoutExpired as exc:
        detail = service.trim_output(str(exc.stderr or exc.stdout or ""))
        suffix = f": {detail}" if detail else ""
        raise AddressedError(f"{' '.join(command)} timed out after {timeout}s{suffix}") from exc
    if result.returncode != 0:
        raise AddressedError(f"{' '.join(command)} failed: {service.trim_output(result.stderr or result.stdout)}")
    try:
        return json.loads(result.stdout or "null")
    except json.JSONDecodeError as exc:
        raise AddressedError(f"{' '.join(command)} returned invalid JSON: {exc}") from exc


def decode_json_stream(raw: str) -> list[Any]:
    values: list[Any] = []
    decoder = json.JSONDecoder()
    index = 0
    raw = raw.strip()
    while index < len(raw):
        while index < len(raw) and raw[index].isspace():
            index += 1
        if index >= len(raw):
            break
        value, index = decoder.raw_decode(raw, index)
        values.append(value)
    return values


def load_json_pages_command(command: list[str], env: dict[str, str] | None = None) -> list[Any]:
    timeout = gh_timeout_seconds()
    try:
        result = subprocess.run(command, capture_output=True, text=True, check=False, env=env, timeout=timeout)
    except subprocess.TimeoutExpired as exc:
        detail = service.trim_output(str(exc.stderr or exc.stdout or ""))
        suffix = f": {detail}" if detail else ""
        raise AddressedError(f"{' '.join(command)} timed out after {timeout}s{suffix}") from exc
    if result.returncode != 0:
        raise AddressedError(f"{' '.join(command)} failed: {service.trim_output(result.stderr or result.stdout)}")
    try:
        return decode_json_stream(result.stdout)
    except json.JSONDecodeError as exc:
        raise AddressedError(f"{' '.join(command)} returned invalid JSON: {exc}") from exc


def parse_github_time(value: str) -> datetime | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def split_repo(full_name: str) -> tuple[str, str]:
    owner, sep, name = full_name.partition("/")
    if not sep or not owner or not name:
        raise AddressedError(f"invalid repo full_name {full_name!r}")
    return owner, name


def gh_repo(repo: str, env: dict[str, str] | None) -> dict[str, Any]:
    data = load_json_command(["gh", "api", f"repos/{repo}"], env=env)
    if not isinstance(data, dict):
        raise AddressedError(f"gh api repos/{repo} returned non-object JSON")
    return data


def gh_recent_items(repo: str, cutoff: datetime, limit: int, env: dict[str, str] | None) -> list[dict[str, Any]]:
    query = f"repo:{repo} updated:>={cutoff.date().isoformat()}"
    data = load_json_pages_command(["gh", "api", "--paginate", "-X", "GET", "search/issues", "-f", f"q={query}"], env=env)
    if not isinstance(data, list):
        raise AddressedError(f"search/issues for {repo} returned non-list JSON")
    items: list[dict[str, Any]] = []
    for page in data:
        if not isinstance(page, dict):
            continue
        raw_items = page.get("items") or []
        if not isinstance(raw_items, list):
            continue
        for item in raw_items:
            if not isinstance(item, dict):
                continue
            items.append(
                {
                    "number": item.get("number", ""),
                    "url": item.get("html_url", ""),
                    "updatedAt": item.get("updated_at", ""),
                    "title": item.get("title", ""),
                    "kind": "pr" if item.get("pull_request") else "issue",
                }
            )
            if limit > 0 and len(items) >= limit:
                return items
    return items


def gh_issue_comments(repo: str, number: str, env: dict[str, str] | None) -> list[dict[str, Any]]:
    data = load_json_pages_command(["gh", "api", "--paginate", f"repos/{repo}/issues/{number}/comments"], env=env)
    if not isinstance(data, list):
        raise AddressedError(f"comment list for {repo}#{number} returned non-list JSON")
    comments: list[dict[str, Any]] = []
    for page in data:
        if isinstance(page, list):
            comments.extend(item for item in page if isinstance(item, dict))
        elif isinstance(page, dict):
            comments.append(page)
    return comments


def repo_installation_id(repo_cfg: dict[str, Any], app_cfg: dict[str, Any]) -> str:
    return str(
        repo_cfg.get("installation_id")
        or app_cfg.get("installation_id")
        or os.environ.get("GC_GITHUB_INSTALLATION_ID", "")
    ).strip()


def gh_env_for_repo(repo_cfg: dict[str, Any], app_cfg: dict[str, Any]) -> dict[str, str]:
    installation_id = repo_installation_id(repo_cfg, app_cfg)
    if not installation_id:
        raise AddressedError("repo address sweep requires repo.installation_id or GC_GITHUB_INSTALLATION_ID")
    token = common.create_installation_token(app_cfg, installation_id)
    env = os.environ.copy()
    env["GH_TOKEN"] = token
    parsed = urllib.parse.urlparse(common.github_web_base())
    host = parsed.netloc
    if host and host != "github.com":
        env["GH_HOST"] = host
        env["GH_ENTERPRISE_TOKEN"] = token
    return env


def build_sweep_payload(
    repo_cfg: dict[str, Any],
    repo_info: dict[str, Any],
    item: dict[str, Any],
    comment: dict[str, Any],
    kind: str,
    installation_id: str,
) -> dict[str, Any]:
    repo = str(repo_cfg.get("full_name", ""))
    owner, name = split_repo(repo)
    issue: dict[str, Any] = {
        "number": item.get("number", ""),
        "title": item.get("title", ""),
        "html_url": item.get("url", ""),
    }
    if kind == "pr":
        issue["pull_request"] = {"url": f"https://api.github.com/repos/{repo}/pulls/{item.get('number', '')}"}
    payload = {
        "action": "created",
        "issue": issue,
        "comment": {
            "id": comment.get("id", ""),
            "body": comment.get("body", ""),
            "html_url": comment.get("html_url", ""),
            "created_at": comment.get("created_at", ""),
            "updated_at": comment.get("updated_at", ""),
            "user": comment.get("user") or {},
        },
        "repository": {
            "id": repo_info.get("id", ""),
            "name": name,
            "full_name": repo,
            "default_branch": repo_info.get("default_branch", ""),
            "owner": {"login": owner},
        },
    }
    if installation_id:
        payload["installation"] = {"id": installation_id}
    return payload


def sweep_comments(limit: int, days: int) -> dict[str, Any]:
    rules = common.load_rules()
    app_cfg = common.load_effective_config().get("app", {})
    if not isinstance(app_cfg, dict):
        app_cfg = {}
    cutoff = datetime.now(timezone.utc) - timedelta(days=days)
    processed: list[dict[str, Any]] = []
    skipped: list[dict[str, Any]] = []
    failures: list[dict[str, Any]] = []
    for repo_cfg in rules.get("repos") or []:
        if not isinstance(repo_cfg, dict) or not repo_cfg.get("addresses"):
            continue
        repo = str(repo_cfg.get("full_name", ""))
        try:
            installation_id = repo_installation_id(repo_cfg, app_cfg)
            if not installation_id:
                skipped.append({"repo": repo, "reason": "installation_missing"})
                continue
            repo_env = gh_env_for_repo(repo_cfg, app_cfg)
            repo_info = gh_repo(repo, repo_env)
            items = gh_recent_items(repo, cutoff, limit, repo_env)
        except Exception as exc:  # noqa: BLE001
            failures.append({"repo": repo, "reason": "item_scan_failed", "detail": service.trim_output(str(exc))})
            continue
        seen_numbers: set[tuple[str, str]] = set()
        for item in items:
            number = str(item.get("number", ""))
            kind = str(item.get("kind", "issue"))
            if not number or (kind, number) in seen_numbers:
                continue
            seen_numbers.add((kind, number))
            updated_at = parse_github_time(str(item.get("updatedAt", "")))
            if updated_at and updated_at < cutoff:
                skipped.append({"repo": repo, "number": number, "kind": kind, "reason": "outside_window"})
                continue
            try:
                comments = gh_issue_comments(repo, number, repo_env)
            except Exception as exc:  # noqa: BLE001
                failures.append({"repo": repo, "number": number, "kind": kind, "reason": "comment_scan_failed", "detail": service.trim_output(str(exc))})
                continue
            for comment in comments:
                created_at = str(comment.get("created_at", ""))
                updated_comment_at = str(comment.get("updated_at", ""))
                if created_at and updated_comment_at and created_at != updated_comment_at:
                    skipped.append({"repo": repo, "number": number, "kind": kind, "comment_id": str(comment.get("id", "")), "reason": "edited_comment"})
                    continue
                payload = build_sweep_payload(repo_cfg, repo_info, item, comment, kind, installation_id)
                extracted = common.extract_addressed_comment_requests(payload, rules)
                if not extracted:
                    continue
                delivery_id = f"sweep-{repo_info.get('id', '')}-{comment.get('id', '')}"
                common.save_delivery(
                    {
                        "delivery_id": delivery_id,
                        "received_at": common.utcnow(),
                        "event": "issue_comment",
                        "source": "addressed-comment-sweep",
                        "payload": payload,
                    }
                )
                outcome = service.process_addressed_comment("issue_comment", delivery_id, payload, app_cfg)
                processed.append({"repo": repo, "number": number, "kind": kind, "comment_id": str(comment.get("id", "")), "outcome": outcome})
    return {
        "status": "failed" if failures else "ok",
        "processed_count": len(processed),
        "skipped_count": len(skipped),
        "failure_count": len(failures),
        "processed": processed,
        "skipped": skipped,
        "failures": failures,
    }


def cmd_router_scan(args: argparse.Namespace) -> int:
    result = service.run_addressed_router(limit=args.limit)
    json_out(result)
    return 1 if args.fail_on_error and result.get("status") != "ok" else 0


def cmd_sweep_comments(args: argparse.Namespace) -> int:
    try:
        result = sweep_comments(limit=args.limit, days=args.days)
    except Exception as exc:  # noqa: BLE001
        result = {"status": "failed", "reason": str(exc), "processed": [], "skipped": [], "failures": []}
    json_out(result)
    return 1 if args.fail_on_error and result.get("status") != "ok" else 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="GitHub addressed-message intake helpers")
    sub = parser.add_subparsers(dest="command", required=True)
    router = sub.add_parser("router-scan", help="dispatch open addressed-message source beads")
    router.add_argument("--limit", type=int, default=50)
    router.add_argument("--fail-on-error", action="store_true")
    router.set_defaults(func=cmd_router_scan)
    sweep = sub.add_parser("sweep-comments", help="reconcile recent GitHub comments with configured addresses")
    sweep.add_argument("--limit", type=int, default=0, help="maximum updated issues/PRs to scan per repo; 0 means all")
    sweep.add_argument("--days", type=int, default=7)
    sweep.add_argument("--fail-on-error", action="store_true")
    sweep.set_defaults(func=cmd_sweep_comments)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())
