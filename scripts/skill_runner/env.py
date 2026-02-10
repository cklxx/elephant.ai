"""Shared environment bootstrap for standalone skill scripts."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path
from typing import Callable

LoadDotenvFn = Callable[..., bool]

_LOADED_PATHS: set[Path] = set()
_INSTALL_ATTEMPTED = False
_LOAD_DOTENV_FN: LoadDotenvFn | None = None


def find_dotenv(start_path: str | os.PathLike[str] | None = None) -> Path | None:
    base = Path(start_path or Path.cwd()).resolve()
    if base.is_file():
        base = base.parent
    for current in (base, *base.parents):
        candidate = current / ".env"
        if candidate.is_file():
            return candidate
    return None


def _resolve_load_dotenv() -> LoadDotenvFn | None:
    global _INSTALL_ATTEMPTED, _LOAD_DOTENV_FN
    if _LOAD_DOTENV_FN is not None:
        return _LOAD_DOTENV_FN

    try:
        from dotenv import load_dotenv

        _LOAD_DOTENV_FN = load_dotenv
        return _LOAD_DOTENV_FN
    except Exception:
        pass

    if _INSTALL_ATTEMPTED:
        return None
    _INSTALL_ATTEMPTED = True

    try:
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "python-dotenv"],
            check=False,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=30,
        )
    except Exception:
        return None

    try:
        from dotenv import load_dotenv

        _LOAD_DOTENV_FN = load_dotenv
        return _LOAD_DOTENV_FN
    except Exception:
        return None


def load_repo_dotenv(
    start_path: str | os.PathLike[str] | None = None, *, override: bool = False
) -> Path | None:
    dotenv_path = find_dotenv(start_path)
    if dotenv_path is None:
        return None

    resolved = dotenv_path.resolve()
    if resolved in _LOADED_PATHS and not override:
        return resolved

    load_dotenv = _resolve_load_dotenv()
    if load_dotenv is None:
        return None

    load_dotenv(dotenv_path=resolved, override=override)
    _LOADED_PATHS.add(resolved)
    return resolved
