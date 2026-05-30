
Read `gc.github.comment_path` and any existing comment metadata from workflow
root metadata. Create or update the body-hash-keyed issue comment through
`{{pack_root}}/assets/scripts/github_api.py comment-create "{{github_issue_url}}" --body-file <gc.github.comment_path>` or
`{{pack_root}}/assets/scripts/github_api.py comment-update`. Update workflow
root metadata with the GitHub comment id and URL. Do not call `gh` directly
except to diagnose a wrapper failure.
