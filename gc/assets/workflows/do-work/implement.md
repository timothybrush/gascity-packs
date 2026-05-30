
Resolve `<source-anchor-id>` using the same rules as `prepare-worktree`. Read `work_dir` from the source anchor, validate that it is an absolute existing git
worktree, set `WORKTREE` to that path, then `cd "$WORKTREE"` before reading or
editing source files. If `work_dir` is missing, invalid, or points at the
launcher checkout, fail this step before editing.

Do not edit files in the launcher checkout. Implement only the owned source
anchor boundary, run sandboxed verification from inside the worktree, and make a
focused commit in the worktree. Leave the source anchor open for
`close-source-anchor`; close only this implementation step when done.
