#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json

import github_intake_common as common


def main() -> int:
    parser = argparse.ArgumentParser(description="Show GitHub intake status")
    parser.add_argument("--json", action="store_true", dest="json_output", help="emit raw JSON")
    args = parser.parse_args()

    snapshot = common.build_status_snapshot(limit=20)
    if args.json_output:
        print(json.dumps(snapshot, indent=2, sort_keys=True))
        return 0

    print(f"city_root: {snapshot['city_root'] or '(unknown)'}")
    print(f"state_root: {snapshot['state_root']}")
    print(f"admin_url: {snapshot['admin_url'] or '(not published yet)'}")
    print(f"webhook_url: {snapshot['webhook_url'] or '(not published yet)'}")

    app = snapshot["config"].get("app", {})
    if app:
        print("app:")
        print(f"  app_id: {app.get('app_id', '(unset)')}")
        print(f"  client_id: {app.get('client_id', '(unset)')}")
        print(f"  slug: {app.get('slug', '(unset)')}")
        print(f"  html_url: {app.get('html_url', '(unset)')}")
        print(f"  webhook_secret_present: {app.get('webhook_secret_present', False)}")
        print(f"  private_key_pem_present: {app.get('private_key_pem_present', False)}")
    else:
        print("app: (not configured)")

    repositories = snapshot["config"].get("repositories", {})
    if repositories:
        print("repository_mappings:")
        for repo_name in sorted(repositories):
            mapping = repositories[repo_name]
            commands = mapping.get("commands", {})
            fix = (commands.get("fix") or {}).get("formula", "(unset)")
            print(f"  {repo_name}:")
            print(f"    target: {mapping.get('target', '(unset)')}")
            print(f"    fix_formula: {fix}")
    else:
        print("repository_mappings: (none)")

    rules = snapshot.get("rules", {})
    print("rules:")
    print(f"  path: {rules.get('path', '(unknown)')}")
    print(f"  count: {rules.get('count', 0)}")
    print(f"  address_repo_count: {rules.get('address_repo_count', 0)}")
    print(f"  address_count: {rules.get('address_count', 0)}")
    if rules.get("error"):
        print(f"  error: {rules.get('error')}")
    ids = rules.get("ids") or []
    if ids:
        print("  ids:")
        for rule_id in ids:
            print(f"    - {rule_id}")

    if snapshot.get("recent_rule_results"):
        print("recent_rule_results:")
        for item in snapshot["recent_rule_results"][:10]:
            print(
                f"  {item.get('result_id')} status={item.get('status')} "
                f"rule={item.get('rule_id')} event={item.get('event')}"
            )
    else:
        print("recent_rule_results: (none)")

    if snapshot.get("recent_address_results"):
        print("recent_address_results:")
        for item in snapshot["recent_address_results"][:10]:
            print(
                f"  {item.get('result_id')} status={item.get('status')} "
                f"reason={item.get('reason', '')} addresses={','.join(item.get('addresses') or [])}"
            )
    else:
        print("recent_address_results: (none)")

    if snapshot["recent_requests"]:
        print("recent_requests:")
        for item in snapshot["recent_requests"][:10]:
            print(
                f"  {item.get('request_id')} status={item.get('status')} "
                f"command={item.get('command')} repo={item.get('repository_full_name')}"
            )
    else:
        print("recent_requests: (none)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
