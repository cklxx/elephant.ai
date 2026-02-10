"""Tests for cc-hooks-setup skill."""

import json
from pathlib import Path

import pytest

from run import remove, setup


@pytest.fixture
def project_dir(tmp_path: Path) -> Path:
    return tmp_path / "project"


def test_setup_new_file(project_dir: Path) -> None:
    result = setup({
        "server_url": "http://localhost:8080",
        "token": "secret123",
        "project_dir": str(project_dir),
    })

    assert result["success"] is True
    assert "配置完成" in result["message"]

    settings_path = project_dir / ".claude" / "settings.local.json"
    assert settings_path.exists()

    data = json.loads(settings_path.read_text())
    assert "hooks" in data
    assert "PostToolUse" in data["hooks"]
    assert "Stop" in data["hooks"]

    command = data["hooks"]["PostToolUse"][0]["hooks"][0]["command"]
    assert "ELEPHANT_HOOKS_URL=http://localhost:8080" in command
    assert "ELEPHANT_HOOKS_TOKEN=secret123" in command
    assert "notify_lark.sh" in command

    hook = data["hooks"]["PostToolUse"][0]["hooks"][0]
    assert hook["async"] is True
    assert hook["timeout"] == 10


def test_setup_merge_existing(project_dir: Path) -> None:
    settings_path = project_dir / ".claude" / "settings.local.json"
    settings_path.parent.mkdir(parents=True, exist_ok=True)
    settings_path.write_text(json.dumps({
        "permissions": {"allow": ["Bash"]},
        "hooks": {"OldHook": [{"hooks": [{"type": "command", "command": "echo old"}]}]},
    }))

    result = setup({
        "server_url": "http://example.com:9090",
        "token": "tok",
        "project_dir": str(project_dir),
    })

    assert result["success"] is True

    data = json.loads(settings_path.read_text())
    # Existing non-hooks config preserved
    assert data["permissions"] == {"allow": ["Bash"]}
    # Hooks replaced (not merged)
    assert "OldHook" not in data["hooks"]
    assert "PostToolUse" in data["hooks"]
    assert "Stop" in data["hooks"]


def test_setup_no_token(project_dir: Path) -> None:
    result = setup({
        "server_url": "http://localhost:8080",
        "project_dir": str(project_dir),
    })

    assert result["success"] is True

    settings_path = project_dir / ".claude" / "settings.local.json"
    data = json.loads(settings_path.read_text())
    command = data["hooks"]["PostToolUse"][0]["hooks"][0]["command"]
    assert "ELEPHANT_HOOKS_URL=http://localhost:8080" in command
    assert "ELEPHANT_HOOKS_TOKEN" not in command


def test_setup_missing_server_url() -> None:
    result = setup({"project_dir": "/tmp/test"})
    assert result["success"] is False
    assert "server_url" in result["error"]


def test_remove_hooks(project_dir: Path) -> None:
    # Setup first
    setup({
        "server_url": "http://localhost:8080",
        "token": "tok",
        "project_dir": str(project_dir),
    })

    settings_path = project_dir / ".claude" / "settings.local.json"
    assert "hooks" in json.loads(settings_path.read_text())

    # Remove
    result = remove({"project_dir": str(project_dir)})
    assert result["success"] is True
    assert "移除" in result["message"]

    # File deleted because only hooks were present
    assert not settings_path.exists()


def test_remove_hooks_preserves_other_settings(project_dir: Path) -> None:
    settings_path = project_dir / ".claude" / "settings.local.json"
    settings_path.parent.mkdir(parents=True, exist_ok=True)
    settings_path.write_text(json.dumps({
        "permissions": {"allow": ["Bash"]},
        "hooks": {"PostToolUse": [{"hooks": []}]},
    }))

    result = remove({"project_dir": str(project_dir)})
    assert result["success"] is True

    data = json.loads(settings_path.read_text())
    assert "hooks" not in data
    assert data["permissions"] == {"allow": ["Bash"]}


def test_remove_no_file(project_dir: Path) -> None:
    result = remove({"project_dir": str(project_dir)})
    assert result["success"] is True
    assert "不存在" in result["message"]


def test_remove_no_hooks(project_dir: Path) -> None:
    settings_path = project_dir / ".claude" / "settings.local.json"
    settings_path.parent.mkdir(parents=True, exist_ok=True)
    settings_path.write_text(json.dumps({"permissions": {"allow": []}}))

    result = remove({"project_dir": str(project_dir)})
    assert result["success"] is True
    assert "无需清理" in result["message"]
