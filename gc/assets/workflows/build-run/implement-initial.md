
If initial_implementation_workflow_id {{initial_implementation_workflow_id}}
is empty, run implement on the approved initial convoy using context path
{{context_path}}, drain policy {{drain_policy}}, and artifact root
{{artifact_root}}. Stop and write final-report.md if implementation fails.

If initial_implementation_workflow_id is non-empty, this is a continuation of
a standalone implement run. Do not skip blindly. First verify all of:
- the referenced bead exists and is a closed graph.v2 workflow
- its title/formula/root key identifies the `implement` formula
- gc.input_convoy_id matches this build-run target convoy
- gc.outcome is pass
- initial_summary_path {{initial_summary_path}} is non-empty, exists, and is
  readable
- the referenced workflow drain manifest, if present, reports succeeded item
  roots or a recoverable summary explains any mismatch

If verification passes, record that the initial implementation was reused and
continue to gap-loop using initial_summary_path as the current implementation
subject. If verification fails, write final-report.md under {{artifact_root}}
with the failed check and stop.
