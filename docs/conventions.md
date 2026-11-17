# Conventions (Phase 1)

## Code style

Prettier default style. Run `npx prettier --write .` before commit.

For Go code: `gofmt` is the single source of truth, run `gofmt -l .` in `hub/` before commit; no deviations.

## Commit convention: Conventional Commits

    <type>(<scope>): <subject>

    [optional body]

Types: feat / fix / docs / style / refactor / perf / test / build / ci / chore

Examples:

    feat(hub): add websocket pool
    fix(rpc): reject non-register frames before registration
    docs(protocol): freeze v0.1 method table
    chore(deps): pin chi v5.2.3

Scope list: ws, rpc, registry, hub, tasks, approvals, protocol, docs, build.

## Phase-1 scope discipline

- Hub implemented in Go (per master plan Go edition), not Node.js.
- protocol/types/protocol.ts still delivered as TS contract artifacts.
- No persistence, no panel UI, no real-agent adapters, no approval timeout.

## Known environment limits

This sandbox has no Node.js and HTTPS is unreachable, so `tsc --noEmit`
and `go mod tidy` (go.sum) must run on the user's dev machine.
