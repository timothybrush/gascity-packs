
Look for the current head-SHA review artifact directory:
`/github/pulls/<owner>/<repo>/<number>/reviews/<head-sha>/`, resolved under
the artifact root with `{{pack_root}}/assets/scripts/artifacts.py path
--override "{{artifact_root}}" --relative
"/github/pulls/<owner>/<repo>/<number>/reviews/<head-sha>/" --directory`.

If a validated review report/comment or waiting human gate already exists for
the same repo, PR number, and head SHA, resume that attempt. If the stored
sticky comment was deleted, create a replacement and update metadata.
