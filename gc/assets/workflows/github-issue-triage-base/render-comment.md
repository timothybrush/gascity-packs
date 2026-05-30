
Read workflow root metadata from `bd show <root-bead-id> --json`. Render one
body-hash-keyed public comment from `gc.github.triage_report_path` to
`<gc.github.triage_dir>/comment.md`. The comment must include a durable
workflow marker for the repo, issue number, and body hash. Use
`{{pack_root}}/assets/scripts/github_reports.py render-triage-comment` so the
structured front-matter fields are followed by the triage report analysis body
for human and future-agent readers. For `security_sensitive` or `priority: p0`,
render only safe public summary text until the human gate approves the final
comment body. Update workflow root metadata with
`gc.github.comment_path=<absolute comment.md path>`. The source issue URL is
{{github_issue_url}}.
