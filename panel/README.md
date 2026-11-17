# panel (Phase-3 front end)

React 19 + TypeScript + Vite + antd v5 + Zustand.

## Dev

    npm install
    npm run dev          # vite on :5173, proxies /panel WS to Hub :8080

Point the browser at http://localhost:5173?token=<hub token> (token is in
the Hub state dir `hub.token`, printed at startup).

## Production (Electron)

    npm run build        # tsc --noEmit + vite build -> dist/
    # electron/main.js spawns the Go Hub with a one-shot token and loads
    # dist/index.html; the endpoint reaches the renderer via preload bridge.

## Structure

- api/frames.ts  panel.* method + frame types (mirrors hub/internal/panel)
- api/client.ts  loopback WS client: call/response correlation, panel.event tap
- api/hub.ts     endpoint discovery (Electron bridge > window global > query)
- shared/store.ts  Zustand store, Hub-authoritative; NO optimistic updates;
  every mutation awaits Hub round-trip then refreshes (Phase-3 5.2)
- app/App.tsx    shell: connection banner, manual reconnect, view switch,
  approval badge + in-app notification on new approvals
- agents/ tasks/ approvals/ events/ intake/ workflows/  views

## Contract notes

- Subscribe once with types:["*"]; events apply incrementally, full snapshot
  is re-pulled after every reconnect (Phase-3 3.4).
- Event buffer caps at 1000 entries, session-only (Phase-3 4.4 decision).
- Approval countdowns: colors at <60s/<10s are a Phase-4 pass together with
  timeout reminders; this Phase-3 skeleton lists and resolves approvals.
