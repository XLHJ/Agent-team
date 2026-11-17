# Phase-2 acceptance record

- Date: 2026-11-13
- Verdict: PASS

## Deliverables

| Item | Result |
|---|---|
| Go Hub rewrite, protocol v0.1 unchanged | hub/ (ws/rpc/registry/tasks/approvals/intake/takeover/server) |
| harness-sdk (Go) | wire, rpc (JSON-RPC client), lifecycle (manual-retry reconnect + recovery), detect (PATH/PATHEXT scan + YAML signatures + shim unwrap), takeover (native / structured-stdio / ConPTY), translate (claude/codex/hermes skeletons), validate (shared schema loader) |
| plugin-sdk (Go) | shared body, embedded + sidecar forms with FormChoiceReason, two runnable samples |
| CLI intake with user confirmation | /intake/candidates, /intake/confirm|ignore|rescan; hub spawns harness-runner (bootstrap token + agent name via env) |
| Reconnect + task recovery | same instance id rebinds; GET /tasks?instance_id&since; TASK_LOST_ON_RECONNECT reported for lost tasks |
| Approval dual-mode interfaces | enum + mode plumbing end to end (register metadata -> approval records) |
| Docs | phase2-cli-signatures.md, phase2-plugin-form-guide.md, phase2-self-test.md |

## Acceptance criteria (Phase-2 list)

| # | Criterion | Result |
|---|---|---|
| 1 | Go Hub passes Phase-1 self-test T1-T12 | PASS (20 assertions, TOTAL fails=0) |
| 2 | harness-sdk and plugin-sdk compile, vet clean | PASS (build + vet zero errors; gofmt clean) |
| 3 | Scan finds >=2 real CLIs and lists candidates | FRAMEWORK PASS: PATH scan + signature + version probe verified end to end (S1); the 5 shipped signatures cover claude-code / codex / hermes / dsh / zcode, actual detection of real installs must be re-run on the Windows dev machine |
| 4 | After user confirm, takeover registers to Hub, panel shows taken-over | PASS at Hub/registry level (S2, cli:fakeagent:* READY); panel display is Phase-3 |
| 5 | Reconnect scenarios 1+2 pass (UNREACHABLE + recovery) | PASS (S4/S5, real process kill + restart) |
| 6 | Both plugin forms compile, register and heartbeat | PASS (embedded-sample / sidecar-sample build; registration path shared with harness-runner which is exercised in S2/S5) |
| 7 | Approval enums + hook points exist on both sides | PASS (wire enums; lifecycle/rpc plumbing; mode recorded on approvals) |
| 8 | Form-choice principle documented | PASS (phase2-plugin-form-guide.md) |

## Verification transcript

- Phase-2 scenarios: 17 PASS / 0 FAIL (TOTAL fails=0) — real hub process,
  real spawned harness-runner, real fakeagent child, kill + restart in S4/S5.
- Phase-1 regression: T1-T12, 20 PASS / 0 FAIL.
- gofmt: zero unformatted files; go vet: clean on all three modules;
  GOOS=windows cross-build: green for hub / harness-sdk / plugin-sdk.

## Known limitations

1. Real-CLI detection (criterion 3) was validated with the fake agent inside
   this sandbox; claude-code / codex / hermes / dsh / zcode YAML entries ship
   but require a Windows machine with the CLIs installed for live detection,
   per-capture event mapping and approval round-trips (Phase-3 adapters).
2. Node.js cannot run in this sandbox (proot/kernel 4.14), so the TS twin of
   wire/lifecycle for Electron hosts is deferred to Phase 3; protocol TS
   types from Phase 1 remain the contract reference.
3. PTY takeover compiles (ConPTY via go-pty) but has no screen parsing —
   observe-only by design, as scoped in Phase 2.

## Key implementation decisions

- Upstream events are sent as requests with discarded replies: v0.1 schemas
  require an id on every frame (validation caught this; Hub rejected id-less
  notifications with -32602).
- Per-task channel: one CLI process per task; terminal event interrupts the
  child instead of waiting for EOF.
- instance_id = cli:<signature>:<hash(name)>: stable across restarts so the
  Hub rebinds and recovery lists previous-life tasks (pid-based ids broke S5).
- Registry: duplicate register while a live connection exists =
  INSTANCE_ID_CONFLICT; a dead record is rebound via Replace.
- Bootstrap token: random per start, written 0600 to the state dir, checked
  on /ws and all Phase-2 endpoints (X-Hub-Token or ?token=).
