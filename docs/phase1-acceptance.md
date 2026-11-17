# Phase-1 acceptance record

- Date: 2026-11-13
- Verdict: PASS (one item deferred to a Node-capable machine, see below)

## Deliverable checklist

| # | Item | Result |
|---|---|---|
| D1 | Protocol v0.1 method list and semantics | Delivered: docs/phase1-protocol-v0.1.md (198 lines, review-ready) |
| D2 | JSON Schema per method | Delivered: 14 schemas; all compile against Draft 2020-12 metaschema; legal samples accepted, illegal samples rejected (14/14) |
| D3 | TypeScript types | Delivered: protocol/types/protocol.ts (202 lines) + strict tsconfig; structure-verified (braces balanced, 14 methods match schemas) |
| D4 | Version and compatibility rules | Delivered: protocol/version.md |
| D5 | Hub skeleton (Go) | Delivered: chi + nhooyr/websocket + jsonschema; go build and go vet clean |
| D6 | Registry and task state machine | Delivered: internal/registry, internal/tasks with documented transition table |
| D7 | Self-test checklist | Delivered: docs/phase1-self-test.md + automated runner cmd/selftest |

## Acceptance criteria

| Criterion | Result |
|---|---|
| tsc --noEmit zero errors | DEFERRED: sandbox has no runnable Node (v18/v22 both abort in proot). tsc must run on dev machine. |
| Every schema accepts legal / rejects illegal | PASS (14/14, automated) |
| Hub starts, GET /health returns 200 | PASS (real binary smoke test) |
| T1-T12 all pass with reproducible records | PASS (20 assertions, automated runner, `TOTAL fails=0`) |
| Method semantics/fields/errors readable without code | PASS (D1) |
| Approval schema + Hub routing done, T9/T10 pass | PASS |

## Known environment limitations

1. No runnable Node.js in this sandbox (Node 18.20.8 and 22.14.0 abort with a
   libuv thread assertion under proot/kernel 4.14). tsc verification is the only
   deferred item.
2. Go module fetch required installing CA certificates from ports.ubuntu.com and
   setting GOPROXY to a reachable mirror; go.sum is committed so later builds are
   offline-capable.
3. Self-test execution notes: automated runner uses an in-process hub; wscat/curl
   manual mode is documented but cannot run here (no Node, no curl binary).

## Scope discipline

No real-agent adapter, no persistence, no panel UI, no approval timeout
automatic decision, no simulated plugin — per Phase-1 scope.
