#!/usr/bin/env python3
"""auto-skill-creation skill — 自动创建新 Skill 脚手架。

生成 SKILL.md + run.py + tests 模板。
"""

from __future__ import annotations

from pathlib import Path
import json
import os
import sys
import time

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

_SKILLS_DIR = Path(os.environ.get("ALEX_SKILLS_DIR", Path(__file__).resolve().parent.parent))


def create(args: dict) -> dict:
    name = args.get("name", "")
    description = args.get("description", "")
    if not name or not description:
        return {"success": False, "error": "name and description are required"}

    skill_dir = _SKILLS_DIR / name
    if skill_dir.exists():
        return {"success": False, "error": f"skill {name} already exists at {skill_dir}"}

    skill_dir.mkdir(parents=True)
    tests_dir = skill_dir / "tests"
    tests_dir.mkdir()

    # SKILL.md
    keywords = args.get("keywords", [name.replace("-", " ")])
    triggers = args.get("triggers", [name.replace("-", "|")])
    (skill_dir / "SKILL.md").write_text(f"""---
name: {name}
description: {description}
triggers:
  intent_patterns:
    - "{"|".join(triggers)}"
  context_signals:
    keywords: {json.dumps(keywords, ensure_ascii=False)}
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# {name}

{description}

## 调用

```bash
python3 skills/{name}/run.py '{{"action":"default", ...}}'
```
""", encoding="utf-8")

    # run.py
    (skill_dir / "run.py").write_text(f'''#!/usr/bin/env python3
"""{name} skill — {description}"""

from __future__ import annotations

from pathlib import Path
import json
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)


def run(args: dict) -> dict:
    action = args.pop("action", "default")
    if action == "default":
        return {{"success": True, "message": "{name} executed", "args": args}}
    return {{"success": False, "error": f"unknown action: {{action}}"}}


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {{}}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
''', encoding="utf-8")

    # tests/__init__.py
    (tests_dir / "__init__.py").write_text("", encoding="utf-8")

    # tests/test_{name}.py
    test_name = name.replace("-", "_")
    (tests_dir / f"test_{test_name}.py").write_text(f'''"""Tests for {name} skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("{test_name}_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

run = _mod.run


class TestRun:
    def test_default_action(self):
        result = run({{}})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({{"action": "invalid"}})
        assert result["success"] is False
''', encoding="utf-8")

    return {
        "success": True,
        "path": str(skill_dir),
        "files": ["SKILL.md", "run.py", "tests/__init__.py", f"tests/test_{test_name}.py"],
        "message": f"Skill「{name}」scaffold created at {skill_dir}",
    }


def run(args: dict) -> dict:
    action = args.pop("action", "create")
    if action == "create":
        return create(args)
    return {"success": False, "error": f"unknown action: {action}"}


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
