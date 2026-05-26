---
name: gh-issue-fix
description: Fix a GitHub issue through triage, planning, build, review, and optional PR publication.
---

# GC GitHub Issue Fix

Use this skill when the user wants the full GitHub issue fix workflow from a
canonical issue URL.

## Workflow

1. Accept only a full URL:
   `https://github.com/<owner>/<repo>/issues/<number>`. Reject shorthand,
   bare numbers, and scheme-less URLs.
2. Launch the targetless `github-issue-fix` graph.v2 formula. Do not pass a
   target, `issue`, `bead_id`, or user-defined `convoy_id`.
3. Always run or reuse `github-issue-triage` first. Continue only for:
   `reproduced + fix` or `not_reproduced + test_hardening`.
4. Generate approved requirements from the issue body and triage report.
5. Run design, decompose, `build-run`, gap-analysis/fix, review/fix, and final
   reporting through the generic gc workflow.
6. Keep one sticky GitHub issue-fix status comment and update it at durable
   transitions.
7. Publish a PR only when `pr_mode` is `draft` or `ready`, after build passes.

`mode=interactive` human-gates design, decomposition/start, and public
publication checkpoints. `mode=autonomous` runs those front-half steps
non-interactively. `pr_mode` is independent of `mode`; autonomous does not imply
PR publication. V0 never merges.

Existing PR reuse is allowed only for PRs authored by the authenticated GitHub
actor and matching the workflow marker, repo/base, and requested mode. Matching
PRs by another author stop as `foreign_pr_exists`.

## Launch Contract

```sh
gc sling <coordinator-target> github-issue-fix --formula \
  --var github_issue_url=https://github.com/<owner>/<repo>/issues/<number> \
  --var mode=interactive \
  --var pr_mode=none \
  --var drain_policy=separate
```

Valid `mode` values are `interactive` and `autonomous`. Valid `pr_mode` values
are `none`, `draft`, and `ready`.
