
Validate graph.v2 target state before side effects.

This step is validation only. Do not edit source files in the launcher checkout,
any existing worktree, or any newly created path. Do not create, modify, or commit source code. Do not run implementation or test-fix loops.

Requirements:
- read reserved `convoy_id` from core graph context
- fail if the target is not a convoy or normalized singleton convoy
- reject legacy `issue`, `bead_id`, or user-supplied `convoy_id` inputs
- validate context_path {{context_path}} with `{{pack_root}}/assets/scripts/validate_context_bundle.py` when set
- reject drain_policy {{drain_policy}} values other than `separate` or `same-session`
- inspect the convoy target branch, defaulting to the repo default branch
- write a run-status artifact if summary_path {{summary_path}} or artifact root is provided
- record push {{push}} and open_pr {{open_pr}}

Close pass only after no workflow roots, drain controls, routed beads,
worktrees, refs, or publish records have been created before validation.
