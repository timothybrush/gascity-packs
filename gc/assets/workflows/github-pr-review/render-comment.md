
Validate the generic review verdict report and map it with
`{{pack_root}}/assets/scripts/github_reports.py review-outcome`:
`pass/none -> approve`, `fail/minor -> comment`, `fail/major -> request_changes`,
and `fail/blocker -> block`. Render the normal PR comment with
`{{pack_root}}/assets/scripts/github_reports.py render-review-comment`.
The source PR URL is {{github_pr_url}}.
