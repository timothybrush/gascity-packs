
Retry-loop container for a small design-review loop. Each attempt runs two focused
review prompts against the current `design.md`, synthesizes their findings,
applies required changes, and stops only when the apply step records
`design_review.verdict=done`.
