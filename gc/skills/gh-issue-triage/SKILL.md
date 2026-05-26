---
name: gh-issue-triage
description: Triage a GitHub issue through the gc GitHub adapter workflow.
---

# GC GitHub Issue Triage

Use this skill when the user wants to triage a GitHub issue from a canonical
issue URL.

## Workflow

1. Accept only a full URL:
   `https://github.com/<owner>/<repo>/issues/<number>`. Reject shorthand,
   bare numbers, and scheme-less URLs.
2. Launch the targetless `github-issue-triage` graph.v2 formula. Do not pass a
   target, `issue`, `bead_id`, or user-defined `convoy_id`.
3. Let the formula snapshot the issue through
   `<pack-root>/assets/scripts/github_api.py`, key triage by
   `sha256(issue.body)`, and reuse the current body-hash result when present.
4. Expect a report with schema `gc.github-issue-triage-report.v1`.
5. Auto-post ordinary triage comments. Human-gate `security_sensitive`,
   `priority: p0`, or explicit `post_mode=human_gate` output.

Triage may gather reproduction artifacts, but it must not create implementation
convoys, commit, push, open PRs, or mutate source branches.

## Launch Contract

```sh
gc sling <coordinator-target> github-issue-triage --formula \
  --var github_issue_url=https://github.com/<owner>/<repo>/issues/<number> \
  --var post_mode=auto
```

Valid `post_mode` values are `auto` and `human_gate`.
