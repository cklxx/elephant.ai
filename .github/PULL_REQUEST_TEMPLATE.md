## Summary
- What changed?
- Why now?

## Architecture Impact
- [ ] This PR keeps the layer direction: `delivery -> app -> domain`, `infra -> domain`, `shared` independent.
- [ ] `internal/domain` has no imports from `internal/app`, `internal/delivery`, or `internal/infra`.
- [ ] If any cross-layer exception is needed, it is added to `configs/arch/exceptions.yaml` with `owner`, `reason`, `expires_at`.

## Scope Checklist
- [ ] No behavior changes to public API/protocol unless explicitly documented.
- [ ] No compatibility shim added.
- [ ] RAG-related code is not reintroduced.

## Validation
- [ ] `make check-arch`
- [ ] `make check-arch-policy`
- [ ] `./dev.sh lint`
- [ ] `./dev.sh test`
- [ ] `go test ./...`

## Notes
- Risks:
- Rollback:
