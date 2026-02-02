# Memory Store Migration (Markdown-Only)

Updated: 2026-02-02

## Summary
- The memory system is now Markdown-only: `~/.alex/memory/MEMORY.md` (long-term) and `~/.alex/memory/memory/YYYY-MM-DD.md` (daily logs).
- There is no database-backed memory store; all durable memory is stored as plain `.md` files.

## Manual Steps (Only If Needed)
1. Stop the server.
2. If you had old memory entries elsewhere, move relevant content into:
   - `~/.alex/memory/MEMORY.md` for durable long-term facts.
   - `~/.alex/memory/memory/YYYY-MM-DD.md` for daily notes.
3. Restart the server; memory search and loading will read the Markdown files directly.

## Notes
- The Markdown files are the source of truth; they can be edited manually and version-controlled.
- Daily logs are append-only; long-term memory should stay concise.
