# Plan: Standardize skills metadata injection and directory-only skills

Owner: cklxx
Date: 2026-02-03
Branch: eli/skills-metadata-standard

## Goals
- Inject full skills metadata into the system prompt using `<available_skills>` XML.
- Enforce directory-only skills (`skill-name/SKILL.md` or `SKILL.mdx`), removing flat `.md` loading.
- Update tests and docs to match the standard layout.

## Status
- [x] Update skills discovery to ignore flat markdown files.
- [x] Update custom skills loader to require directory layout.
- [x] Add `<available_skills>` XML generator.
- [x] Inject metadata into system prompt.
- [x] Update docs and tier rules.
- [x] Update tests.
- [x] Run fmt/vet/test.
- [ ] Commit and merge.
