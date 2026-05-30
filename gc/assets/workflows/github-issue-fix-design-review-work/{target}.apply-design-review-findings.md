
Read the current attempt synthesis and update `gc.github.design_path` in place.

If the global verdict is `approve`:
- set design front matter `status: approved`
- add or refresh a short accepted-risks note when relevant
- stamp root metadata `gc.github.design_review_status=approved`
- close this bead with `design_review.verdict=done`

If the global verdict is `iterate`:
- apply all required changes to `design.md`
- keep front matter `status: draft`
- stamp root metadata `gc.github.design_review_status=iterating`
- close this bead with `design_review.verdict=iterate`

For every attempt, write:
- `design-after.md`
- `design.diff`
- `apply-summary.md`

Close with `gc.outcome=pass`, `design_review.verdict=done|iterate`,
`design_review.output_path=<apply-summary path>`, and
`gc.continuation_group=design-review-fixes`. Do not edit source files.
