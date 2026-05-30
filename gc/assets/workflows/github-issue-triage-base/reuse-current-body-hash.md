
Read the current step bead metadata, get `gc.root_bead_id`, then read
workflow root metadata with `bd show <root-bead-id> --json`. Use
`gc.github.repo`, `gc.github.number`, `gc.github.body_hash`,
`gc.github.snapshot_path`, and `gc.github.triage_dir` as the context index.
If any required key is missing, hard-fail and report that the snapshot handoff
metadata is incomplete.

If a validated `triage-report.md` and current comment metadata already exist
for the same repo, issue number, and body hash, return that run after refreshing
source metadata. If the stored comment was deleted, create a replacement from
the existing rendered comment and update workflow root metadata.
