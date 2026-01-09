# Error Experience

This index avoids merge conflicts by keeping each log entry and summary item in its own file.

## Layout

- Log entries: `docs/error-experience/entries/` (one file per entry, append-only).
- Summary index: `docs/error-experience/summary.md` (static, no per-change edits).
- Summary entries: `docs/error-experience/summary/entries/` (one file per summary item).
- Retention: when entries exceed 6, move older entries into summary entries and delete their files.

## Entry format

- Filename: `YYYY-MM-DD-short-slug.md`
- Content:
  - `Error: ...`
  - `Remediation: ...`

## Summary entry format

- Filename: `YYYY-MM-DD-short-slug.md`
- Content:
  - `Summary: ...`
  - `Remediation: ...`
