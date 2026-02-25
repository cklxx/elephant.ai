"""Base class for all Python skills."""

from __future__ import annotations

import json
import subprocess
import sys
import traceback
from dataclasses import dataclass, field
from typing import Any


@dataclass
class SkillResult:
    """Structured result returned by a skill execution."""

    success: bool = True
    data: Any = None
    message: str = ""
    error: str = ""

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {"success": self.success}
        if self.data is not None:
            d["data"] = self.data
        if self.message:
            d["message"] = self.message
        if self.error:
            d["error"] = self.error
        return d


class Skill:
    """Base class every Python skill inherits from.

    Subclasses must:
      - set ``name`` class attribute
      - implement ``execute(self, action, **kwargs) -> SkillResult``
    """

    name: str = ""

    # ── public API ───────────────────────────────────────────────

    def execute(self, action: str, **kwargs: Any) -> SkillResult:
        """Override this in your skill."""
        raise NotImplementedError

    # ── helpers for subclasses ───────────────────────────────────

    @staticmethod
    def sh(cmd: str, *, timeout: int = 120, cwd: str | None = None) -> str:
        """Run a shell command and return stdout. Raises on non-zero exit."""
        r = subprocess.run(
            cmd,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=cwd,
        )
        if r.returncode != 0:
            raise RuntimeError(
                f"command failed (rc={r.returncode}): {cmd}\n"
                f"stderr: {r.stderr.strip()}"
            )
        return r.stdout.strip()

    @staticmethod
    def read_file(path: str) -> str:
        with open(path, encoding="utf-8") as f:
            return f.read()

    @staticmethod
    def write_file(path: str, content: str) -> None:
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    @staticmethod
    def ok(data: Any = None, message: str = "") -> SkillResult:
        return SkillResult(success=True, data=data, message=message)

    @staticmethod
    def fail(error: str) -> SkillResult:
        return SkillResult(success=False, error=error)

    # ── entry point ──────────────────────────────────────────────

    @classmethod
    def run(cls) -> None:
        """Parse args, execute, print JSON result."""
        instance = cls()
        try:
            args = cls._parse_input()
            action = args.pop("action", "default")
            result = instance.execute(action, **args)
        except Exception as exc:
            result = SkillResult(
                success=False,
                error=f"{type(exc).__name__}: {exc}",
            )
        # Always output valid JSON to stdout
        json.dump(result.to_dict(), sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        sys.stdout.flush()
        sys.exit(0 if result.success else 1)

    # ── internal ─────────────────────────────────────────────────

    @staticmethod
    def _parse_input() -> dict[str, Any]:
        """Read input from CLI arg or stdin."""
        if len(sys.argv) > 1:
            raw = sys.argv[1]
        elif not sys.stdin.isatty():
            raw = sys.stdin.read()
        else:
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            print(
                json.dumps({"success": False, "error": f"invalid JSON input: {exc}"}),
                file=sys.stdout,
            )
            sys.exit(1)
