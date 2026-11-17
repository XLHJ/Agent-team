# Phase-3 acceptance record

- Date: 2026-11-13
- Verdict: PASS (panel front end delivered as source; build deferred to a
  Node-capable machine)

## What Phase 3 covers

Panel as a read-only projection + command entry over the Hub. All state is
Hub-authoritative; the panel never owns business state.

## Backend deliverables (verified in this sandbox)

| Item | Result |
|---|---|
| `/panel` loopback WS channel, token + origin guarded, single session enforced | selftest3 P3-01..03 PASS |
| panel.* method set: subscribe/unsubscribe, agents.list, tasks.list/get/create/cancel, approvals.list/respond, intake.*, workflows.* | dispatch table in hub/internal/panel; P3-04..08 PASS |
| Business error codes over panel channel (APPROVAL_NOT_FOUND, INSTANCE_NOT_FOUND, -32601 unknown method) | P3-05..07 PASS |
| panel.event push with wildcard subscription | P3-08 PASS |
| Workflow engine (save/start/runs/run/cancel/delete) driving sequential chains with the frozen handoff format | selftest4 W1..W5 PASS |
| SQLite persistence for approvals + workflows (approvals store, recovery re-arms timers) | store package; approval recovery logged at hub start |

## Front end deliverables (source-complete, build on dev machine)

- panel/: React 19 + TS + Vite + antd v5 + Zustand.
- api/client.ts: JSON-RPC over loopback WS, id-correlated calls, panel.event tap.
- shared/store.ts: Hub-authoritative store; no optimistic updates; event
  buffer capped at 1000 (session-only).
- Views: Agent kanban / task board (create + cancel + detail drawer) /
  approvals (pending + resolved, approve / deny / JSON modify) / event
  stream (type + body filters) / intake (confirm / ignore / rescan) /
  workflows (list + start + delete + quick sequential-chain creation).
- app/App.tsx: dark shell, connection banner with manual reconnect (one
  attempt per click, full re-snapshot after reconnect), approval badge and
  in-app notification.
- electron/main.js + preload.cjs: spawns the Go Hub with a one-shot token,
  parses the bound port from stdout, bridges the endpoint to the renderer.

## Dev machine checklist (needs Node)

1. `cd panel && npm install && npm run build` (tsc --noEmit must pass).
2. `npm run dev` with Hub on :8080, open `http://localhost:5173/?token=<hub.token>`
   (token in the Hub state dir `hub.token`), then walk: agents list -> create
   task -> watch progress -> detail drawer -> approvals -> intake -> events.
3. Electron packaging (electron-builder config) is intentionally not included:
   it belongs to the Phase-4+ ship pass.

## Known deferrals (recorded, not hidden)

- Approval countdown colors (<60s/<10s), timeout popups: Phase 4 (approval
  lifecycle pass) together with agentic-ui HITL components.
- Workflow editor canvas (ReactFlow/ELK): Phase 5 per the phase reports.
- Event persistence: intentionally session-only per the Phase-3 4.4 decision.
