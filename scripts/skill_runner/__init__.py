"""skill_runner — Lightweight base for elephant.ai Python skills.

Each skill is a standalone Python script (``run.py``) that:
  1. Accepts input as CLI JSON arg or stdin JSON
  2. Executes its workflow (API calls, subprocess, file I/O)
  3. Prints a JSON result to stdout

Usage in a skill's ``run.py``::

    from skill_runner import Skill, SkillResult

    class CalendarSkill(Skill):
        name = "calendar-management"

        def execute(self, action: str, **kwargs) -> SkillResult:
            if action == "create":
                return self.create_event(**kwargs)
            ...

    if __name__ == "__main__":
        CalendarSkill.run()
"""

from skill_runner.base import Skill, SkillResult

__all__ = ["Skill", "SkillResult"]
