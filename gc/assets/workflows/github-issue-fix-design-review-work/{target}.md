
Validate that `gc.github.design_path` exists and its front matter has
`status: approved`. If not, fail this step with `gc.outcome=fail` and
`gc.failure_class=hard`.

On success, update this step and the workflow root with:
- `gc.github.design_path=<absolute design.md path>`
- `gc.github.design_status=approved`
- `gc.github.design_review_status=approved`
- `gc.github.design_review_dir=<absolute review dir>`
- `gc.outcome=pass`

Close this sink step with `gc.outcome=pass`. Downstream `decompose` must not
need to know whether the review came from the base two-lane loop or a local
override.
