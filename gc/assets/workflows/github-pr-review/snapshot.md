
Resolved GitHub PR URL: {{github_pr_url}}
Resolved artifact root override: {{artifact_root}} (empty means use the rig artifact root)
Resolved context path: {{context_path}} (empty means no extra context bundle)
Resolved post mode: {{post_mode}}

Artifact root semantics:
- Resolve the durable artifact root with
  `{{pack_root}}/assets/scripts/artifacts.py root --override "{{artifact_root}}"`.
- Paths shown with a leading slash, such as
  `/github/pulls/<owner>/<repo>/<number>/source.json`, are
  artifact-root-relative, not filesystem-root absolute.
- Resolve concrete files with
  `{{pack_root}}/assets/scripts/artifacts.py path --override "{{artifact_root}}" --relative "/github/pulls/<owner>/<repo>/<number>/source.json" --mkdir-parents`.
- Store absolute resolved artifact paths in bead metadata. Do not infer source
  identity, workflow identity, readiness, or routing from artifact paths.

Validate the PR URL with
`{{pack_root}}/assets/scripts/github_api.py parse-url "{{github_pr_url}}" --kind pull`,
reject shorthand inputs, then fetch the current PR source snapshot with
`{{pack_root}}/assets/scripts/github_api.py pr-snapshot "{{github_pr_url}}"`.

Write the returned snapshot JSON to
artifact-root-relative path `/github/pulls/<owner>/<repo>/<number>/source.json`.
Then create or refresh the canonical GitHub source bead using this v0 contract:

- Source beads are non-runnable index/cache beads. Do not route the source bead,
  assign it, depend on it, or use it as a readiness gate.
- Lookup uses object identity only:
  `bd list --metadata-field gc.kind=github_source --metadata-field gc.github.kind=pull --metadata-field gc.github.repo=<owner>/<repo> --metadata-field gc.github.number=<number> --status open,in_progress,closed --limit 1 --json`.
- Write `source-metadata.json` with flat string metadata:
  `gc.kind=github_source`, `gc.github.kind=pull`,
  `gc.github.repo=<owner>/<repo>`, `gc.github.number=<number>`,
  `gc.github.url=<canonical_url>`, `gc.github.title=<title>`,
  `gc.github.state=<state>`, `gc.github.author=<author>`,
  `gc.github.body_hash=<body_hash>`,
  `gc.github.head_sha=<head_sha>`, `gc.github.head_ref=<head_ref>`,
  `gc.github.head_repo=<head_repo>`, `gc.github.base_ref=<base_ref>`,
  `gc.github.base_repo=<base_repo>`,
  `gc.github.snapshot_path=<absolute source.json path>`,
  `gc.github.updated_at=<updated_at>`.
- If no bead exists, create it with
  `bd create "GitHub PR source: <owner>/<repo>#<number>" --type task --labels gc.github-source,gc.github-pr --external-ref <canonical_url> --metadata @source-metadata.json`.
- If a bead exists, refresh it with
  `bd update <source-bead-id> --external-ref <canonical_url> --metadata @source-metadata.json`.

Only `gc.github.head_sha` controls head-SHA-keyed PR review reuse.
