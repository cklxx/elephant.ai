"""Shared environment bootstrap for standalone skill scripts."""

from __future__ import annotations

import os
from pathlib import Path
from typing import Callable

LoadDotenvFn = Callable[..., bool]

_LOADED_PATHS: set[Path] = set()
_LOAD_DOTENV_FN: LoadDotenvFn | None = None


def _home_alex_root(path: Path) -> Path | None:
    for current in (path, *path.parents):
        if current.name == "skills" and current.parent.name == ".alex":
            return current.parent
    return None


def _iter_path_to_root(path: Path, *, stop: Path | None = None):
    current = path
    while True:
        yield current
        if stop is not None and current == stop:
            return
        parent = current.parent
        if parent == current:
            return
        current = parent


def _iter_search_roots(start_path: str | os.PathLike[str] | None = None):
    base = Path(start_path or Path.cwd()).resolve()
    if base.is_file():
        base = base.parent

    roots = [base]

    repo_root = os.environ.get("ALEX_REPO_ROOT", "").strip()
    if repo_root:
        roots.append(Path(repo_root).expanduser().resolve())

    roots.append(Path.cwd().resolve())

    seen: set[Path] = set()
    for root in roots:
        if root in seen:
            continue
        seen.add(root)
        yield root


def find_dotenv(start_path: str | os.PathLike[str] | None = None) -> Path | None:
    for root in _iter_search_roots(start_path):
        stop_at = _home_alex_root(root)
        for current in _iter_path_to_root(root, stop=stop_at):
            candidate = current / ".env"
            if candidate.is_file():
                return candidate
    return None


def _simple_load_dotenv(*, dotenv_path: str | os.PathLike[str], override: bool = False) -> bool:
    """Minimal dotenv loader that avoids runtime package installation."""
    path = Path(dotenv_path)
    if not path.is_file():
        return False

    loaded = False
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].strip()
        if "=" not in line:
            continue

        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()

        if not key:
            continue
        if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
            value = value[1:-1]
        if override or key not in os.environ:
            os.environ[key] = value
            loaded = True
    return loaded


def _resolve_load_dotenv() -> LoadDotenvFn | None:
    global _LOAD_DOTENV_FN
    if _LOAD_DOTENV_FN is not None:
        return _LOAD_DOTENV_FN

    try:
        from dotenv import load_dotenv

        _LOAD_DOTENV_FN = load_dotenv
        return _LOAD_DOTENV_FN
    except Exception:
        _LOAD_DOTENV_FN = _simple_load_dotenv
        return _LOAD_DOTENV_FN


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
