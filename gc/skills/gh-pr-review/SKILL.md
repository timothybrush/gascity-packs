---
name: gh-pr-review
description: Review a GitHub pull request through the gc GitHub adapter workflow.
---

# GC GitHub PR Review

Use this skill when the user wants a non-mutating review of a GitHub pull
request from a canonical PR URL.

## Workflow

1. Accept only a full URL:
   `https://github.com/<owner>/<repo>/pull/<number>`. Reject shorthand, bare
   numbers, and scheme-less URLs.
2. Launch the targetless `github-pr-review` graph.v2 formula. Do not pass a
   target, `issue`, `bead_id`, or user-defined `convoy_id`.
3. Let the formula snapshot the PR through
   `<pack-root>/assets/scripts/github_api.py` and key review attempts by PR
   head SHA.
4. Delegate review judgment to the generic targetless `review` formula.
5. Post or update one sticky normal PR comment for the current head SHA.

V0 posts normal PR comments only. It must not submit formal GitHub review
events, mutate code, push commits, amend contributor branches, or open follow-up
PRs.

## Launch Contract

```sh
gc sling <coordinator-target> github-pr-review --formula \
  --var github_pr_url=https://github.com/<owner>/<repo>/pull/<number> \
  --var post_mode=human_gate
```

Valid `post_mode` values are `human_gate` and `auto`. `human_gate` is the
default.
