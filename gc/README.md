# GC Mayor Workflow Pack

This pack provides a convoy-first planning and implementation workflow for Gas
City work:

- `gc.mayor` is the user-facing coordinator skill. It gathers requirements,
  writes designs, decomposes approved plans into convoys/beads, and discovers
  runnable workflow formulas.
- Formula workflows opt into discovery with `[catalog]` metadata. The mayor
  discovers them with `gc formula catalog --json`, inspects them with
  `gc formula show <name> --json`, and launches them with `gc sling`.
- Cataloged formulas cover implementation, build loops, targetless reports, and
  GitHub adapter workflows. Helper/base formulas remain out of the catalog.

Import it with the `gc` binding:

```toml
[imports.gc]
source = "../gascity-packs/gc"
```

Run the mayor skill to plan and coordinate work:

```text
Use skill gc.mayor
```

Discover formula workflows from the active rig/city context:

```sh
gc formula catalog --json
gc formula show implement --json
```

Then launch implementation against an approved implementation convoy:

```sh
gc sling gc.run-operator <convoy-id> --on implement \
  --var context_path=<optional-context-yaml> \
  --var drain_policy=separate
```

Every formula in this pack uses `contract = "graph.v2"`. Targeted formulas take
the core-injected reserved convoy target; they do not declare `issue`,
`bead_id`, or `convoy_id` variables. `drain_policy=separate` is the standalone
default. Use `same-session` only when preserving one shared worktree and
conversation is explicitly desired and core shared drain support is available.

The pack ships providerless rig role agents under `gc/roles`. Standalone use
requires both imports: the top-level `gc` import for formulas and the mayor
skill, plus a `gc/roles` import on each target rig that should run work. A city
that imports only the formulas can read the mayor skill, but default formula
steps will not have rig-local `gc.*` role agents to route to.

Import the roles pack for each target rig so work runs in the target
repository. By default the agents inherit the city/workspace provider; advanced
users can patch individual roles to a specific provider without overriding
formulas:

```toml
[[rigs]]
name = "my-repo"

[rigs.imports.gc]
source = "/path/to/gc/roles"

[[rigs.patches]]
agent = "gc.implementation-worker"
provider = "your-provider"
```

Launch the formulas from the target rig context, or pass your normal
`--rig <target-rig>` selection so `gc.run-operator` resolves to the rig-local
role from `gc/roles`.

Default formula routes use these qualified targets: `gc.run-operator`,
`gc.requirements-planner`, `gc.design-author`, `gc.task-decomposer`,
`gc.issue-triager`, `gc.design-implementation-reviewer`,
`gc.design-test-risk-reviewer`, `gc.review-synthesizer`,
`gc.implementation-worker`, `gc.gap-analyst`, `gc.implementation-reviewer`,
and `gc.publisher`.

## Customizing Workflow Behavior

There are two intended customization levels.

For the basic path, shadow a step asset with the same relative path in your
city or local pack import. Formula step bodies live under
`assets/workflows/<formula>/<step-id>.md`; Gas City resolves these paths through
the normal import/layer search path. To customize GitHub issue triage without
changing the workflow graph, add this file in your city assets:

```text
assets/workflows/github-issue-triage-base/write-triage-report.md
```

A real local override might add instructions like:

```markdown
Apply our repository triage policy before writing the report:

- If the issue touches `internal/api/` or generated API schema files, read
  `engdocs/architecture/api-control-plane.md` and
  `engdocs/contributors/huma-usage.md`.
- Treat missing reproduction evidence as `needs-info` unless the report
  includes a failing test, stack trace, or linked CI artifact.
- Label user-visible regressions as `p1` only when current `origin/main` is
  affected; historical release-only reports are `p2` unless data loss is
  plausible.
- Include exact commands run, relevant file paths, and any GitHub labels that
  should be applied.
```

For advanced customization, override the formula step whose ID matches the
extension point you want to replace. Copy the bundled formula into your local
pack or city formula layer, preserve the vars and metadata you still need, and
replace only the step block. For example, to replace the standard local review
with an N-wide review plus synthesis pipeline, override `build-run` step
`review-loop` or the lower-level `review` step `write-report` to expand to your
own review quorum:

```toml
[[steps]]
id = "review-loop"
title = "Run local review quorum"
needs = ["gap-loop"]
description_file = "../assets/workflows/build-run/review-loop.md"
expand = "company-review-n-wide"
metadata = { "gc.run_target" = "gc.review-synthesizer" }
```

Keep sink contracts stable when downstream steps depend on them. For review and
gap-analysis, write the same `schema: gc.verdict-report.v1` report with
`verdict: pass|fail`; for GitHub adapter workflows, preserve the documented
`gc.github.*` metadata on the workflow root bead.

Common formula step IDs:

| Workflow | Best basic asset | Best advanced step IDs |
| --- | --- | --- |
| Design review | `assets/workflows/design-review/design-review.md` | `design-review`, `finalize` |
| Plan and decompose issue fixes | `assets/workflows/github-issue-fix-base/generate-requirements.md` | `generate-requirements`, `design`, `design-review`, `decompose` |
| Build implementation convoys | `assets/workflows/build-run/implement-initial.md` | `implement-initial`, `gap-loop`, `review-loop`, `publish` |
| Direct implementation | `assets/workflows/implement/prepare.md` | `prepare`, `drain-separate`, `drain-same-session`, `wait-for-drain`, `summarize` |
| Per-item implementation | `assets/workflows/do-work/implement.md` | `prepare-worktree`, `implement`, `close-source-anchor` |
| Gap analysis | `assets/workflows/gap-analysis/write-report.md` | `validate-context`, `write-report` |
| Review | `assets/workflows/review/write-report.md` | `validate-context`, `write-report` |
| GitHub issue triage | `assets/workflows/github-issue-triage-base/write-triage-report.md` | `snapshot`, `write-triage-report`, `human-gate-sensitive-output`, `post-comment`, `finalize` |
| GitHub PR review | `assets/workflows/github-pr-review/run-review.md` | `snapshot`, `run-review`, `human-gate-comment`, `post-comment`, `finalize` |
| Bug report flow | `assets/workflows/bug-report-flow/investigation-synthesis.md` | `reported-build-repro`, `main-repro`, `investigation-synthesis`, `dispatch-implementation` |
| Bug hunt | `assets/workflows/bug-hunt/hunter-fanout.md` | `prepare-hunters`, `hunter-fanout`, `synthesize-findings`, `finalize` |

By default artifacts go under the target rig:

```text
<rig-root>/.gc/plans/<plan-slug>/
  requirements.md
  design.md
  tasks.md
  context.yaml
  build/final-report.md
```

The mayor may use a different artifact root when the user explicitly asks for
one. The same `<plan-slug>/` structure should be used under the override root.

The mayor uses `assets/scripts/create_beads_from_tasks.py` after approving a
task plan. The script requires Python 3 with PyYAML available, invokes `gc bd --rig
<target_rig>` for runnable beads, invokes `gc convoy --rig <target_rig>` for
convoy heads and membership, and records the created mapping in `tasks.md`.

The `tasks.md` payload uses nested `convoys[]` and `beads[]`; `epics[]` is a
hard validation error. `convoys[].dependencies` expands to runnable bead edges
from the upstream terminal runnable beads to the downstream root runnable beads.

Context bundles are YAML or JSON files with:

```yaml
items:
  - name: Requirements
    path: requirements.md
    description: Product requirements and acceptance criteria.
```

Each item has only `name`, `path`, and `description`. Validate with:

```sh
python3 <pack-root>/assets/scripts/validate_context_bundle.py context.yaml --allow-root <artifact-root>
```

Gap-analysis and review reports use `schema: gc.verdict-report.v1` front matter
with `verdict: pass|fail`. Validate with:

```sh
python3 <pack-root>/assets/scripts/validate_verdict_report.py report.md --kind review
```

## GitHub Adapter Workflows

The GitHub workflows are targetless `graph.v2` formulas. They accept only full
canonical URLs:

```text
https://github.com/<owner>/<repo>/issues/<number>
https://github.com/<owner>/<repo>/pull/<number>
```

Launch triage:

```sh
gc sling gc.run-operator github-issue-triage --formula \
  --var github_issue_url=https://github.com/<owner>/<repo>/issues/<number>
```

Customize triage behavior by setting `triage_rubric_path` to a Markdown
rubric/prompt, either at launch or in a rig's `formula_vars`. The rubric can
carry project-specific label policy, priority semantics, investigation rules,
or a richer report skeleton. The base formula still owns the metadata handoff,
report schema, validator, security gate, and comment protocol.

For deeper customization, create a local `github-issue-triage` formula that
extends `github-issue-triage-base` and overrides only `write-triage-report`.
That replacement step can inline an expansion or delegate to another workflow,
but it should still read the pack-owned GitHub metadata from the workflow root
bead and write the same `gc.github-issue-triage-report.v1` sink metadata.

Launch PR review:

```sh
gc sling gc.run-operator github-pr-review --formula \
  --var github_pr_url=https://github.com/<owner>/<repo>/pull/<number> \
  --var post_mode=human_gate
```

Launch issue fix:

```sh
gc sling gc.run-operator github-issue-fix --formula \
  --var github_issue_url=https://github.com/<owner>/<repo>/issues/<number> \
  --var mode=interactive \
  --var pr_mode=none \
  --var drain_policy=separate
```

GitHub API calls go through wrapper scripts in
`<pack-root>/assets/scripts/`. Formulas should call those wrappers, not `gh`
directly, except when diagnosing wrapper failures.
