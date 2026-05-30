
Resume the latest active nonterminal issue-fix run by default. If no active run
exists, create one under
artifact-root-relative path
`/github/issues/<owner>/<repo>/<number>/fix/<run-id>/`, resolved with
`{{pack_root}}/assets/scripts/artifacts.py path --override "{{artifact_root}}"
--relative "/github/issues/<owner>/<repo>/<number>/fix/<run-id>/" --mkdir-parents
--directory`. If the issue body hash changed while a run is active, run/reuse
triage for the new hash and ask the human whether to continue the old run with
updated context or start fresh.
