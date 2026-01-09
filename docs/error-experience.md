# Error Experience
> Last updated: 2026-01-09

This index avoids merge conflicts by keeping each log entry in its own file.

## Layout

- Log entries: `docs/error-experience/entries/` (one file per entry, append-only).
- Summary: `docs/error-experience/summary.md` (update when rolling older entries).
- Retention: when entries exceed 6, roll older ones into the summary and delete their files.

## Entry format

- Filename: `YYYY-MM-DD-short-slug.md`
- Content:
  - `Error: ...`
  - `Remediation: ...`
