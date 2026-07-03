from __future__ import annotations

import hashlib
import subprocess
import sys
import textwrap
import tomllib

import pytest

import validate_registry

_PACK_TOML_BYTES = b'[pack]\nname = "cass"\nschema = 2\n'
_README_BYTES = b"CASS docs\n"


def run_git(root, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def _init_pack_repo(root) -> str:
    run_git(root, "init")
    run_git(root, "config", "user.email", "test@example.com")
    run_git(root, "config", "user.name", "Test User")
    pack_dir = root / "cass"
    pack_dir.mkdir()
    (pack_dir / "pack.toml").write_bytes(_PACK_TOML_BYTES)
    (pack_dir / "README.md").write_bytes(_README_BYTES)
    run_git(root, "add", "cass")
    run_git(root, "commit", "-m", "add cass")
    return run_git(root, "rev-parse", "HEAD")


def test_source_pack_path_accepts_tree_urls() -> None:
    source = "https://github.com/gastownhall/gascity-packs/tree/main/cass"

    assert validate_registry.source_pack_path(source) == "cass"


def test_validate_tree_url_source_checks_pack_toml_name(tmp_path) -> None:
    pack_dir = tmp_path / "cass"
    pack_dir.mkdir()
    (pack_dir / "pack.toml").write_text(
        textwrap.dedent(
            """\
            [pack]
            name = "wrong"
            schema = 2
            """
        ),
        encoding="utf-8",
    )
    registry = tmp_path / "registry.toml"
    registry.write_text(
        textwrap.dedent(
            """\
            schema = 1

            [[pack]]
            name = "cass"
            description = "CASS session search pack."
            source = "https://github.com/gastownhall/gascity-packs/tree/main/cass"
            source_kind = "git"

              [[pack.release]]
              version = "0.1.0"
              ref = "main"
              commit = "d3617d1319a1206ac85f69ba024ec395c49c6f4b"
              hash = "sha256:9849675daa3ba8a792fc1c68c727542936400687d529e5d4d231afde29d4a341"
              description = "Initial CASS session-search pack release."
            """
        ),
        encoding="utf-8",
    )

    errors = validate_registry.validate(registry)

    assert "cass: registry name does not match cass/pack.toml name 'wrong'" in errors


def test_pack_content_hash_uses_relative_paths_modes_and_blob_hashes(tmp_path) -> None:
    commit = _init_pack_repo(tmp_path)

    manifest = "\n".join(
        sorted(
            [
                f"README.md 0644 {hashlib.sha256(_README_BYTES).hexdigest()}",
                f"pack.toml 0644 {hashlib.sha256(_PACK_TOML_BYTES).hexdigest()}",
            ]
        )
    ).encode("utf-8")
    expected = "sha256:" + hashlib.sha256(manifest).hexdigest()

    assert validate_registry.git_pack_content_hash(tmp_path, commit, "cass") == expected


def test_resolve_commit_returns_full_lowercase_sha(tmp_path) -> None:
    head = _init_pack_repo(tmp_path)

    resolved = validate_registry.resolve_commit(tmp_path, "HEAD")

    assert validate_registry.COMMIT_RE.fullmatch(resolved)
    assert resolved == head


def test_compute_pack_hash_matches_validator_and_raises_when_absent(tmp_path) -> None:
    commit = _init_pack_repo(tmp_path)

    computed = validate_registry.compute_pack_hash(tmp_path, "cass", commit)

    assert computed == validate_registry.git_pack_content_hash(tmp_path, commit, "cass")
    assert validate_registry.HASH_RE.fullmatch(computed)
    with pytest.raises(ValueError):
        validate_registry.compute_pack_hash(tmp_path, "missing", commit)


def test_render_pack_entry_parses_and_carries_computed_hash(tmp_path) -> None:
    commit = _init_pack_repo(tmp_path)
    content_hash = validate_registry.compute_pack_hash(tmp_path, "cass", commit)

    block = validate_registry.render_pack_entry(
        name="cass",
        description="CASS session search pack.",
        source="https://github.com/gastownhall/gascity-packs/tree/main/cass",
        version="0.1.0",
        ref="main",
        commit=commit,
        content_hash=content_hash,
        release_description="Initial CASS session-search pack release.",
    )

    parsed = tomllib.loads(block)
    entry = parsed["pack"][0]
    release = entry["release"][0]
    assert entry["name"] == "cass"
    assert entry["source_kind"] == "git"
    assert release["commit"] == commit
    assert release["hash"] == content_hash
    assert validate_registry.HASH_RE.fullmatch(release["hash"])


def test_render_pack_entry_escapes_quotes_in_descriptions(tmp_path) -> None:
    commit = _init_pack_repo(tmp_path)
    content_hash = validate_registry.compute_pack_hash(tmp_path, "cass", commit)

    description = 'Has a "quote" and a \\ backslash'
    block = validate_registry.render_pack_entry(
        name="cass",
        description=description,
        source="https://github.com/gastownhall/gascity-packs/tree/main/cass",
        version="0.1.0",
        ref="main",
        commit=commit,
        content_hash=content_hash,
        release_description="Initial release.",
    )

    parsed = tomllib.loads(block)
    assert parsed["pack"][0]["description"] == description


def test_cli_compute_happy_path(tmp_path, monkeypatch, capsys) -> None:
    commit = _init_pack_repo(tmp_path)
    registry = tmp_path / "registry.toml"
    registry.write_bytes(b"schema = 1\n")
    monkeypatch.setattr(sys, "argv", [
        "validate_registry", str(registry), "--compute", "cass", "--commit", commit,
    ])

    result = validate_registry.main()

    assert result == 0
    out, _ = capsys.readouterr()
    expected = validate_registry.git_pack_content_hash(tmp_path, commit, "cass")
    assert out.strip() == expected


def test_cli_emit_entry_missing_required_args_exit_2(tmp_path, monkeypatch, capsys) -> None:
    registry = tmp_path / "registry.toml"
    registry.write_bytes(b"schema = 1\n")
    monkeypatch.setattr(sys, "argv", [
        "validate_registry", str(registry), "--emit-entry", "cass",
    ])

    result = validate_registry.main()

    assert result == 2
    _, err = capsys.readouterr()
    assert "emit-entry failed" in err


def test_cli_mutual_exclusion_guard(tmp_path, monkeypatch, capsys) -> None:
    registry = tmp_path / "registry.toml"
    registry.write_bytes(b"schema = 1\n")
    monkeypatch.setattr(sys, "argv", [
        "validate_registry", str(registry), "--compute", "cass", "--emit-entry", "cass",
    ])

    result = validate_registry.main()

    assert result == 2
    _, err = capsys.readouterr()
    assert "mutually exclusive" in err


def test_cli_pack_name_traversal_rejected(tmp_path, monkeypatch, capsys) -> None:
    registry = tmp_path / "registry.toml"
    registry.write_bytes(b"schema = 1\n")
    monkeypatch.setattr(sys, "argv", [
        "validate_registry", str(registry), "--compute", "../../x",
    ])

    result = validate_registry.main()

    assert result == 1
    _, err = capsys.readouterr()
    assert "invalid pack name" in err
