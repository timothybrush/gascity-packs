
Only after build passes implementation, gap-analysis, and review, publish
according to PR mode {{pr_mode}}: `draft` opens or updates a draft PR; `ready`
opens or updates a ready-for-review PR. Resolve the authenticated actor through
`{{pack_root}}/assets/scripts/github_api.py actor` and reuse an existing PR only
when the workflow marker, repo/base, authenticated author, and requested mode
are compatible. If a matching marker belongs to another author, record
`foreign_pr_exists` and ask the human how to proceed. V0 never merges.
