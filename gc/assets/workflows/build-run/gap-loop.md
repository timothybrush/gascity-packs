
Run gap-analysis against the current implementation subject. For a reused
initial implementation, the first subject is initial_summary_path
{{initial_summary_path}}. Otherwise use the summary or diff produced by
implement-initial.

If {{artifact_root}} already contains a valid gap-analysis report for that
same subject and no later fix iteration has superseded it, reuse it as the
current gap verdict and record the reuse. Otherwise run the public
gap-analysis formula.

On failed verdict, create a fix convoy from the report findings and run
implement until pass or max_iterations {{max_iterations}} is exhausted. Preserve
context path {{context_path}} and drain policy {{drain_policy}} for fix work.
