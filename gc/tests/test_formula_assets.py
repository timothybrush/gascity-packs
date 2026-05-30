from __future__ import annotations

import os
import pathlib
import tomllib
import unittest


FORMULAS = {
    "adopt-pr-review",
    "build-run",
    "bug-hunt",
    "bug-report-flow",
    "bug-report-implementation",
    "design-review",
    "do-work",
    "do-work-item",
    "fix-convoy",
    "gap-analysis",
    "github-issue-fix",
    "github-issue-fix-base",
    "github-issue-fix-design-review-work",
    "github-issue-triage-base",
    "github-issue-triage",
    "github-pr-review",
    "implement",
    "post-merge-pr-review",
    "publish",
    "review",
    "same-session-implement",
}

ROLE_AGENTS = {
    "design-author",
    "design-implementation-reviewer",
    "design-test-risk-reviewer",
    "gap-analyst",
    "implementation-reviewer",
    "implementation-worker",
    "issue-triager",
    "publisher",
    "requirements-planner",
    "review-synthesizer",
    "run-operator",
    "task-decomposer",
}

CATALOG_FORMULAS = {
    "adopt-pr-review",
    "build-run",
    "bug-hunt",
    "bug-report-flow",
    "design-review",
    "gap-analysis",
    "github-issue-fix",
    "github-issue-triage",
    "github-pr-review",
    "implement",
    "review",
}


def load_formula(root: pathlib.Path, name: str) -> dict:
    return tomllib.loads((root / "formulas" / f"{name}.formula.toml").read_text(encoding="utf-8"))


def merged_steps(parent_steps: list[dict], child_steps: list[dict]) -> list[dict]:
    result = list(parent_steps)
    positions = {step["id"]: idx for idx, step in enumerate(result)}
    for step in child_steps:
        idx = positions.get(step["id"])
        if idx is None:
            positions[step["id"]] = len(result)
            result.append(step)
        else:
            result[idx] = step
    return result


def resolve_formula(root: pathlib.Path, name: str, seen: tuple[str, ...] = ()) -> dict:
    if name in seen:
        raise AssertionError(f"circular formula extends: {' -> '.join((*seen, name))}")
    data = load_formula(root, name)
    parents = data.get("extends", [])
    if not parents:
        return data

    merged: dict = {
        "formula": data["formula"],
        "description": data.get("description", ""),
        "version": data.get("version", 1),
        "contract": data.get("contract", ""),
        "target_required": data.get("target_required"),
        "vars": {},
        "steps": [],
    }
    for parent in parents:
        parent_data = resolve_formula(root, parent, (*seen, name))
        if not merged["contract"]:
            merged["contract"] = parent_data.get("contract", "")
        if merged["target_required"] is None:
            merged["target_required"] = parent_data.get("target_required")
        merged["vars"].update(parent_data.get("vars", {}))
        merged["steps"].extend(parent_data.get("steps", []))

    merged["vars"].update(data.get("vars", {}))
    merged["steps"] = merged_steps(merged["steps"], data.get("steps", []))
    if data.get("description"):
        merged["description"] = data["description"]
    return merged


def effective_formula_text(root: pathlib.Path, name: str) -> str:
    data = load_formula(root, name)
    chunks = []
    for parent in data.get("extends", []):
        chunks.append(effective_formula_text(root, parent))
    formula_path = root / "formulas" / f"{name}.formula.toml"
    chunks.append(formula_path.read_text(encoding="utf-8"))
    for node in formula_nodes(data):
        description_file = node.get("description_file")
        if description_file:
            chunks.append((formula_path.parent / description_file).resolve().read_text(encoding="utf-8"))
    return "\n".join(chunks)


def formula_nodes(data: dict) -> list[dict]:
    nodes = list(data.get("steps", []))
    nodes.extend(data.get("template", []))
    for template in data.get("template", []):
        nodes.extend(template.get("children", []))
    return nodes


def node_description(root: pathlib.Path, node: dict) -> str:
    description_file = node.get("description_file")
    if description_file:
        return (root / "formulas" / description_file).resolve().read_text(encoding="utf-8")
    return node["description"]


class FormulaAssetTests(unittest.TestCase):
    def test_expected_formula_set_is_convoy_first(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        paths = sorted((root / "formulas").glob("*.formula.toml"))

        self.assertEqual({path.name.removesuffix(".formula.toml") for path in paths}, FORMULAS)
        for path in paths:
            data = tomllib.loads(path.read_text(encoding="utf-8"))
            name = path.name.removesuffix(".formula.toml")
            self.assertEqual(data["formula"], name)
            self.assertEqual(data["contract"], "graph.v2")
            var_names = set(data.get("vars", {}))
            self.assertNotIn("issue", var_names)
            self.assertNotIn("bead_id", var_names)
            self.assertNotIn("convoy_id", var_names, f"{path.name} must not redeclare reserved convoy_id")

    def test_expected_role_agents_are_providerless(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        roles_pack = tomllib.loads((root / "roles" / "pack.toml").read_text(encoding="utf-8"))
        paths = sorted((root / "roles" / "agents").glob("*/agent.toml"))

        self.assertEqual(roles_pack["pack"]["name"], "gc-roles")
        self.assertEqual({path.parent.name for path in paths}, ROLE_AGENTS)
        for path in paths:
            data = tomllib.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(data["scope"], "rig")
            self.assertTrue(data["fallback"])
            self.assertNotIn("provider", data, f"{path} must inherit the city/workspace provider by default")
            self.assertTrue((path.parent / "prompt.template.md").is_file())
        self.assertIn(root / "roles" / "agents" / "run-operator" / "agent.toml", paths)

    def test_role_agent_prompts_include_graph_claim_protocol(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        shared_lines = (
            root / "roles" / "prompts" / "shared" / "gc-role-worker.md.tmpl"
        ).read_text(encoding="utf-8").splitlines()
        expected = "\n".join(shared_lines[1:-1]).strip()

        for fragment in (
            "GC_CLAIM",
            "`gc hook` is the only permitted discovery source",
            "bd update \"$WORK_ID\" --claim --json",
            "CLAIM_REJECTED",
            "gc runtime drain-ack",
            "gc.continuation_group",
            "gc.scope_role=teardown",
            "check for more routed work before draining",
            "running the same `GC_CLAIM` block again",
        ):
            with self.subTest(fragment=fragment):
                self.assertIn(fragment, expected)

        for agent_name in ROLE_AGENTS:
            prompt = root / "roles" / "agents" / agent_name / "prompt.template.md"
            with self.subTest(agent=agent_name):
                self.assertEqual(prompt.read_text(encoding="utf-8").strip(), expected)

    def test_formula_run_targets_are_backed_by_providerless_role_agents(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        for path in sorted((root / "formulas").glob("*.formula.toml")):
            data = tomllib.loads(path.read_text(encoding="utf-8"))
            for step in data.get("steps", []):
                target = step.get("metadata", {}).get("gc.run_target", "")
                if not target:
                    continue
                with self.subTest(formula=path.name, step=step["id"], target=target):
                    self.assertTrue(target.startswith("gc."))
                    self.assertIn(target.removeprefix("gc."), ROLE_AGENTS)
                    self.assertNotIn("{{", target)
                    self.assertNotIn("workflows.", target)

    def test_formula_catalog_metadata_marks_user_runnable_workflows(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        catalog_names: set[str] = set()
        for path in sorted((root / "formulas").glob("*.formula.toml")):
            data = tomllib.loads(path.read_text(encoding="utf-8"))
            name = path.name.removesuffix(".formula.toml")
            catalog = data.get("catalog")
            if catalog is None:
                continue
            with self.subTest(formula=name):
                self.assertEqual(catalog["name"], name)
                self.assertIsInstance(catalog.get("description"), str)
                self.assertGreater(len(catalog["description"].strip()), 0)
            catalog_names.add(name)

        self.assertEqual(catalog_names, CATALOG_FORMULAS)

    def test_formula_node_descriptions_delegate_to_shadowable_assets(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        for formula_path in sorted((root / "formulas").glob("*.formula.toml")):
            formula = formula_path.name.removesuffix(".formula.toml")
            data = tomllib.loads(formula_path.read_text(encoding="utf-8"))
            for node in formula_nodes(data):
                with self.subTest(formula=formula, node=node["id"]):
                    self.assertNotIn("description", node)
                    description_file = node.get("description_file")
                    self.assertEqual(
                        description_file,
                        f"../assets/workflows/{formula}/{node['id']}.md",
                    )
                    self.assertTrue((formula_path.parent / description_file).resolve().is_file())

    def test_implement_formula_uses_core_drain_steps(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        data = tomllib.loads((root / "formulas" / "implement.formula.toml").read_text(encoding="utf-8"))

        self.assertNotIn("infra_target", data["vars"])
        self.assertNotIn("hard_target", data["vars"])
        self.assertNotIn("worker_target", data["vars"])

        step_ids = [step["id"] for step in data["steps"]]
        self.assertEqual(step_ids, ["prepare", "drain-separate", "drain-same-session", "wait-for-drain", "summarize"])

        separate = data["steps"][1]
        same = data["steps"][2]
        self.assertEqual(data["steps"][0]["metadata"]["gc.run_target"], "gc.run-operator")
        self.assertEqual(separate["metadata"]["gc.run_target"], "gc.implementation-worker")
        self.assertEqual(separate["condition"], "{{drain_policy}} == separate")
        self.assertEqual(separate["drain"]["context"], "separate")
        self.assertEqual(separate["drain"]["formula"], "do-work")
        self.assertEqual(separate["drain"]["member_access"], "exclusive")
        self.assertEqual(same["metadata"]["gc.run_target"], "gc.implementation-worker")
        self.assertEqual(same["condition"], "{{drain_policy}} == same-session")
        self.assertEqual(same["drain"]["context"], "shared")
        self.assertEqual(same["drain"]["formula"], "do-work-item")
        self.assertEqual(same["drain"]["member_access"], "exclusive")
        self.assertTrue(same["drain"]["item"]["single_lane"])
        self.assertEqual(same["drain"]["on_item_failure"], "skip_remaining")
        self.assertEqual(data["steps"][3]["metadata"]["gc.run_target"], "gc.run-operator")
        self.assertEqual(data["steps"][4]["metadata"]["gc.run_target"], "gc.run-operator")
        wait = node_description(root, data["steps"][3])
        for fragment in (
            "Wait only on the core drain control bead",
            "gc.drain_manifest.v1",
            "Do not wait for or inspect downstream steps",
            "summarize, workflow-finalize, or root workflow closure",
            "cannot progress\nuntil this bead closes",
            "close only this wait step",
        ):
            with self.subTest(step="wait-for-drain", fragment=fragment):
                self.assertIn(fragment, wait)

        helper = tomllib.loads((root / "formulas" / "same-session-implement.formula.toml").read_text(encoding="utf-8"))
        self.assertEqual(helper["steps"][0]["drain"]["member_access"], "exclusive")

    def test_implement_prepare_is_validation_only(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        data = tomllib.loads((root / "formulas" / "implement.formula.toml").read_text(encoding="utf-8"))
        prepare = next(step for step in data["steps"] if step["id"] == "prepare")

        for fragment in (
            "validation only",
            "Do not edit source files in the launcher checkout",
            "Do not create, modify, or commit source code",
            "Do not run implementation or test-fix loops",
        ):
            with self.subTest(fragment=fragment):
                self.assertIn(fragment, node_description(root, prepare))

    def test_item_implementation_formulas_route_role_agents(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]

        do_work = tomllib.loads((root / "formulas" / "do-work.formula.toml").read_text(encoding="utf-8"))
        self.assertNotIn("infra_target", do_work["vars"])
        self.assertNotIn("hard_target", do_work["vars"])
        self.assertEqual(do_work["steps"][0]["metadata"]["gc.run_target"], "gc.run-operator")
        self.assertEqual(do_work["steps"][1]["metadata"]["gc.run_target"], "gc.implementation-worker")
        self.assertEqual(do_work["steps"][2]["metadata"]["gc.run_target"], "gc.run-operator")

        do_work_item = tomllib.loads((root / "formulas" / "do-work-item.formula.toml").read_text(encoding="utf-8"))
        self.assertNotIn("infra_target", do_work_item["vars"])
        self.assertNotIn("hard_target", do_work_item["vars"])
        self.assertEqual(do_work_item["steps"][0]["metadata"]["gc.run_target"], "gc.implementation-worker")

    def test_do_work_formula_requires_persisted_item_worktree(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        do_work = tomllib.loads((root / "formulas" / "do-work.formula.toml").read_text(encoding="utf-8"))
        steps = {step["id"]: step for step in do_work["steps"]}

        prepare = node_description(root, steps["prepare-worktree"])
        for fragment in (
            "current step bead metadata",
            "gc.root_bead_id",
            "gc.input_convoy_id",
            "gc.synthetic_kind",
            "gc.drain_member_id",
            "worktrees/<source-anchor-id>",
            "git worktree add",
            "bd update <source-anchor-id> --set-metadata work_dir=",
            "Do not edit source files in the launcher checkout",
        ):
            with self.subTest(step="prepare-worktree", fragment=fragment):
                self.assertIn(fragment, prepare)

        implement = node_description(root, steps["implement"])
        for fragment in (
            "Read `work_dir` from the source anchor",
            "cd \"$WORKTREE\"",
            "fail this step before editing",
            "Do not edit files in the launcher checkout",
            "Leave the source anchor open",
        ):
            with self.subTest(step="implement", fragment=fragment):
                self.assertIn(fragment, implement)

        close_source = node_description(root, steps["close-source-anchor"])
        for fragment in (
            "Read `work_dir` from the source anchor",
            "close only `<source-anchor-id>`",
            "bd show <source-anchor-id> --json",
            "status=closed",
            "gc.outcome=pass",
            "if either check fails",
            "anchor before closing this step",
        ):
            with self.subTest(step="close-source-anchor", fragment=fragment):
                self.assertIn(fragment, close_source)

    def test_wrapper_formulas_route_role_agents(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]

        build_run = tomllib.loads((root / "formulas" / "build-run.formula.toml").read_text(encoding="utf-8"))
        self.assertNotIn("infra_target", build_run["vars"])
        self.assertNotIn("hard_target", build_run["vars"])
        self.assertEqual(build_run["steps"][0]["metadata"]["gc.run_target"], "gc.implementation-worker")
        self.assertEqual(build_run["steps"][1]["metadata"]["gc.run_target"], "gc.gap-analyst")
        self.assertEqual(build_run["steps"][2]["metadata"]["gc.run_target"], "gc.implementation-reviewer")
        self.assertEqual(build_run["steps"][3]["metadata"]["gc.run_target"], "gc.run-operator")
        self.assertEqual(build_run["steps"][4]["metadata"]["gc.run_target"], "gc.publisher")

        issue_fix = resolve_formula(root, "github-issue-fix")
        self.assertNotIn("infra_target", issue_fix["vars"])
        self.assertNotIn("hard_target", issue_fix["vars"])
        route_by_step = {step["id"]: step["metadata"]["gc.run_target"] for step in issue_fix["steps"]}
        self.assertEqual(route_by_step["snapshot"], "gc.run-operator")
        self.assertEqual(route_by_step["triage"], "gc.issue-triager")
        self.assertEqual(route_by_step["triage-gate"], "gc.run-operator")
        self.assertEqual(route_by_step["resume-or-create-run"], "gc.run-operator")
        self.assertEqual(route_by_step["update-status-started"], "gc.run-operator")
        self.assertEqual(route_by_step["generate-requirements"], "gc.requirements-planner")
        self.assertEqual(route_by_step["design"], "gc.design-author")
        self.assertEqual(route_by_step["design-review"], "gc.review-synthesizer")
        self.assertEqual(route_by_step["decompose"], "gc.task-decomposer")
        self.assertEqual(route_by_step["build"], "gc.implementation-worker")
        self.assertEqual(route_by_step["publish-pr"], "gc.publisher")
        self.assertEqual(route_by_step["finalize"], "gc.run-operator")

        design_review = load_formula(root, "github-issue-fix-design-review-work")
        self.assertEqual(set(design_review.get("vars", {})), {"mode"})
        design_review_text = effective_formula_text(root, "github-issue-fix-design-review-work")
        for target in (
            "gc.run-operator",
            "gc.design-implementation-reviewer",
            "gc.design-test-risk-reviewer",
            "gc.review-synthesizer",
        ):
            with self.subTest(formula="github-issue-fix-design-review-work", target=target):
                self.assertIn(f'"gc.run_target" = "{target}"', design_review_text)
        self.assertNotIn("reviewer_one_target", design_review_text)
        self.assertNotIn("reviewer_two_target", design_review_text)
        self.assertNotIn("synthesizer_target", design_review_text)

    def test_base_formulas_do_not_ship_private_workflow_language(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]

        self.assertFalse((root / "formulas" / "release.formula.toml").exists())
        for path in sorted((root / "formulas").glob("*.formula.toml")):
            raw_text = path.read_text(encoding="utf-8")
            text = raw_text.lower()
            with self.subTest(formula=path.name):
                self.assertNotIn("homebrew", text)
                self.assertNotIn("goreleaser", text)
                self.assertNotIn("gastownhall/gascity", text)
                self.assertNotIn("bugflow", text)
                self.assertNotIn("Ralph", raw_text)

    def test_report_formulas_are_targetless_and_report_only(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        for name in ("gap-analysis", "review"):
            data = tomllib.loads((root / "formulas" / f"{name}.formula.toml").read_text(encoding="utf-8"))
            self.assertEqual(data["mode"], "report")
            self.assertFalse(data["target_required"])
            self.assertEqual([step["id"] for step in data["steps"]], ["validate-context", "write-report"])

    def test_github_adapter_formulas_are_targetless_url_adapters(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        expected = {
            "github-issue-triage": ("github_issue_url", {"artifact_root", "post_mode", "triage_rubric_path"}),
            "github-pr-review": ("github_pr_url", {"artifact_root", "context_path", "post_mode"}),
            "github-issue-fix": ("github_issue_url", {"artifact_root", "mode", "pr_mode", "drain_policy"}),
        }
        for name, (url_var, optional_vars) in expected.items():
            with self.subTest(name=name):
                data = resolve_formula(root, name)
                self.assertEqual(data["contract"], "graph.v2")
                self.assertFalse(data["target_required"])
                self.assertTrue(data["vars"][url_var]["required"])
                self.assertEqual(set(data["vars"]) - {url_var}, optional_vars)
                text = effective_formula_text(root, name)
                self.assertIn("{{pack_root}}/assets/scripts/github_api.py", text)
                self.assertNotIn("{{pack_root}}" + "/scripts/", text)

    def test_github_adapter_formulas_define_source_bead_contract(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        expected = {
            "github-issue-triage": ("issue", "gc.github.body_hash"),
            "github-issue-fix": ("issue", "gc.github.body_hash"),
            "github-pr-review": ("pull", "gc.github.head_sha"),
        }
        required_common = {
            "bd list --metadata-field gc.kind=github_source",
            "bd create",
            "bd update",
            "--external-ref",
            "gc.github.kind",
            "gc.github.repo",
            "gc.github.number",
            "gc.github.url",
            "gc.github.snapshot_path",
            "Do not route the source bead",
        }

        for name, (github_kind, idempotency_key) in expected.items():
            with self.subTest(name=name):
                text = effective_formula_text(root, name)
                for fragment in required_common:
                    self.assertIn(fragment, text)
                self.assertIn(f"gc.github.kind={github_kind}", text)
                self.assertIn(idempotency_key, text)

    def test_github_adapter_formulas_define_artifact_root_semantics(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        for name in ("github-issue-triage", "github-issue-fix", "github-pr-review"):
            with self.subTest(name=name):
                text = effective_formula_text(root, name)
                self.assertIn("{{pack_root}}/assets/scripts/artifacts.py root", text)
                self.assertIn("{{pack_root}}/assets/scripts/artifacts.py path", text)
                self.assertIn("artifact-root-relative", text)
                self.assertIn("not filesystem-root absolute", text)
                self.assertIn("gc.github.snapshot_path=<absolute source.json path>", text)

    def test_github_issue_triage_formula_requires_human_readable_analysis(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        text = effective_formula_text(root, "github-issue-triage")
        self.assertIn("human-readable analysis body", text)
        self.assertIn("## Summary", text)
        self.assertIn("## Evidence", text)
        self.assertIn("## Recommendation", text)
        self.assertIn("render-triage-comment", text)

    def test_github_issue_triage_uses_workflow_metadata_as_context_index(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        text = effective_formula_text(root, "github-issue-triage")

        required_fragments = {
            "workflow root metadata",
            "gc.root_bead_id",
            "gc.github.source_bead_id",
            "gc.github.triage_dir",
            "bd show <root-bead-id> --json",
            "bd update <root-bead-id>",
            "Read `gc.github.snapshot_path`",
            "Do not write a separate triage context file",
        }
        for fragment in required_fragments:
            with self.subTest(fragment=fragment):
                self.assertIn(fragment, text)
        self.assertNotIn("triage-context.json", text)

    def test_github_issue_triage_supports_rubric_override_without_protocol_override(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        data = resolve_formula(root, "github-issue-triage")
        text = effective_formula_text(root, "github-issue-triage")

        self.assertIn("triage_rubric_path", data["vars"])
        self.assertEqual(data["vars"]["triage_rubric_path"]["default"], "")
        self.assertIn("{{triage_rubric_path}}", text)
        self.assertIn("Optional rubric/prompt override path", text)
        self.assertIn("report behavior, not the metadata protocol", text)
        self.assertIn("must not override", text)
        self.assertIn("gc.github-issue-triage-report.v1", text)

    def test_github_issue_triage_human_gate_uses_runtime_metadata_in_step_body(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        data = resolve_formula(root, "github-issue-triage")

        gate = next(step for step in data["steps"] if step["id"] == "human-gate-sensitive-output")
        self.assertNotIn("condition", gate)
        self.assertIn("gc.github.triage_priority", node_description(root, gate))
        self.assertIn("no-op gate", node_description(root, gate))

    def test_all_declared_formula_vars_are_rendered_into_graph_text(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        for path in sorted((root / "formulas").glob("*.formula.toml")):
            data = tomllib.loads(path.read_text(encoding="utf-8"))
            text = effective_formula_text(root, path.name.removesuffix(".formula.toml"))
            for var_name in data.get("vars", {}):
                with self.subTest(formula=path.name, var=var_name):
                    if data.get("type") == "expansion":
                        self.assertTrue(
                            f"{{{{{var_name}}}}}" in text or f"{{{var_name}}}" in text,
                            f"{path.name} must render {var_name} as a runtime or expansion variable",
                        )
                    else:
                        self.assertIn(f"{{{{{var_name}}}}}", text)

    def test_check_scripts_are_executable_and_portable(self) -> None:
        root = pathlib.Path(__file__).resolve().parents[1]
        scripts = sorted((root / "assets" / "scripts" / "checks").glob("*.sh"))

        self.assertEqual(
            [script.name for script in scripts],
            ["design-review-approved.sh", "gap-analysis-approved.sh", "implementation-review-approved.sh"],
        )
        for script in scripts:
            text = script.read_text(encoding="utf-8")
            self.assertTrue(os.access(script, os.X_OK), f"{script} must be executable")
            self.assertNotIn("/data/projects", text)
            self.assertNotIn("gascity-packs-worktrees", text)


if __name__ == "__main__":
    unittest.main()
