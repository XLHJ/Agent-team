# Phase 4 acceptance: approval engine

- Status: delivered and verified 2026-11-13
- Scope: engine + persistence + HTTP surface. Panel UI views remain gated on the Phase-3/4 graphic spec (chapter 9) and are intentionally NOT coded.

## Delivered

- hub/internal/store: SQLite via modernc.org/sqlite (pure Go, CGO-free).
  - open.go: Open/Schema/Audit/AuditList
  - approvals.go: SaveApproval/ResolveApproval/GetApproval (DB-first commit, ErrAlreadyDone on re-resolve)
  - list.go: ListApprovals with status/instance/task filters
  - cleanup.go: CleanupOld retention sweep
  - MarshalJSON flattens sql.NullString to bare JSON values (Phase-1/2 shape preserved)
- hub/internal/approvalsv2/engine.go: lifecycle PENDING -> {resolved_approved, resolved_denied, resolved_modified, timeout_denied}
  - Request(): timeout clamp [30s,30min] with warn log, default 5min
  - timeoutFire(): fail-closed deny + reason "timeout", audit approval.timeout
  - Resolve(): DB-first, human decision, exemption grant (approve+exempt, risk!=high)
  - Recover(): boot restore of PENDING rows, remaining = timeout_ms - (now - created_at), <=0 resolves immediately
  - ClearExempts(): task-terminal cleanup + audit exemption.cleared
  - Cleanup(): 30-day retention + audit approval.cleanup
  - validate.go: Modify must be a JSON object, <=64KB
  - Exempt auto-approve: reason "exempted", audit exemption.applied, panel notified
- hub/internal/panel: Phase-3 backend channel
  - /panel WS: token guard (401), Origin whitelist (403, "null" + localhost:5173 + empty), single-panel (409)
  - panel.* methods: subscribe/unsubscribe/agents.list/tasks.list|get|create|cancel/approvals.list|respond/intake.candidates|confirm|ignore
  - panel.event push via internal Bus (engine events bridged in main)
  - hub.port file (0600) written at startup
- Wiring: router approval.requested -> engine (params_preview passthrough, exempted auto-respond); server /approvals filters; main wires store, Recover, timeout routing, OnTerminal exemption cleanup, 24h retention sweep, hub.port

## Verification

- gofmt: zero diff
- go vet: all hub modules green
- T1-T12 regression (cmd/selftest): 20 assertions, fails=0
  - T9 updated to precise terminal status resolved_approved (doc 2.1)
- S0-S5 regression (cmd/selftest2): fails=0 (real hub + spawned runner + kill/restart)
- P3-01..P3-08 (cmd/selftest3): fails=0
- GOOS=windows cross-compile: PASS (all hub packages, re-run 2026-11-15)

## Not delivered (documented deferral)

- Approval center UI views (waiting queue, history, detail, JSON editor, popups) - gated on graphic spec
- panel.workflows.* - Phase 5
- DAG, parallel stages - Phase 5 out of scope by design
