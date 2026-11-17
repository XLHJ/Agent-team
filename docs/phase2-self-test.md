# Phase-2 self-test

## Scope

Phase 2 = integration layer: intake of real CLI agents, the shared SDKs and
reconnect recovery. Protocol stays frozen at v0.1; the Phase-1 checklist
(T1-T12, `cmd/selftest`) runs unchanged as regression.

## Prerequisites

Build all binaries from the repo root (`agent-control-plane/`):

    cd hub                && go build -o hub ./cmd/hub
    cd hub                && go build -o hub.selftest2 ./cmd/selftest2
    cd harness-sdk        && go build -o harness-runner ./cmd/harness-runner
    cd harness-sdk        && go build -o fakeagent ./cmd/fakeagent

Run from `hub/cmd/selftest2`:

    cd hub/cmd/selftest2 && ../../hub.selftest2

The runner boots a real Hub process on a free port, spawns harness-runner via
the intake confirm endpoint and drives the fake CLI agent (line-based
structured-stdio stand-in with `--version`).

## Scenarios

| # | Assertion | Verified behavior |
|---|---|---|
| S0 | hub healthy | `/health` 200 with bootstrap token enabled |
| S1 | scan detects fakeagent 0.9.0 | PATH scan + YAML signature + version probe |
| S2 | confirm accepted | `POST /intake/confirm` spawns harness-runner (process tree owned by Hub) |
| S2 | instance cli:fakeagent:* READY | runner registers `cli:<sig>:<hash>`, stable across restarts |
| S2 | caps | capability declaration by takeover method; report.generate declared for stdio |
| S3 | dispatch accepted | capability-guarded dispatch to a taken-over CLI |
| S3 | task completed | structured-stdio loop: dispatch -> progress -> completed, report persisted |
| S3 | raw_output echoes prompt | prompt flowed through the translator to the agent and back |
| S3 | report non-empty | report.generate: harness produces non-empty report on task.completed |
| S4 | slow task dispatched/running | prompt `SLOW=<ms>` exercises the async path |
| S4 | disconnect accepted | user-facing disconnect kills the runner process tree |
| S4 | record UNREACHABLE | registry flips conn_status, record kept for rebind |
| S4 | task stays non-terminal | dispatch/running tasks are never auto-failed on drop |
| S5 | same instance id rebinds | restarted runner reuses `cli:<sig>:<hash>`; registry Replace, no duplicate |
| S5 | TASK_LOST_ON_RECONNECT | on rebind the runner reports failed TASK_LOST_ON_RECONNECT for tasks lost across the restart |

Latest run: **17 PASS / 0 FAIL (TOTAL fails=0)**.

## Phase-1 regression

    cd hub/cmd/selftest && go run . ../../../protocol/schemas

Latest run: **T1-T12, 20 assertions PASS / 0 FAIL**.

## Notes

- The fake agent lives in `harness-sdk/cmd/fakeagent`; its signature entry is
  `harness-sdk/detect/signatures/fakeagent.yaml` (moved into the production
  signature dir so the Hub scan finds it via PATH).
- Windows cross-compile `GOOS=windows go build ./...` is green for hub,
  harness-sdk and plugin-sdk (ConPTY path compiles against x/sys/windows).
