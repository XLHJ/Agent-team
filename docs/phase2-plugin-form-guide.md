# Phase-2 plugin forms: embedded vs sidecar

Decision order (mandatory, mirrors the Phase-2 report):

1. If the agent host exposes an embeddable extension point (MCP server,
   local plugin API, module loader), use the embedded form. Lowest cost,
   finest control.
   - Go host: import `plugin-sdk/embedded`, call `RegisterEmbeddedPlugin`.
   - Electron/Node host: use the TS twin of wire+lifecycle (same frames;
     `plugin-sdk/ts` is the Phase-3 home for it).
2. If embedding requires modifying the agent itself, non-public APIs or
   special permissions, fall back to the sidecar form
   (`plugin-sdk/sidecar`, `RunSidecarPlugin`). The adapter owns the IPC
   to the agent.
3. If both are viable and cost is close, or feasibility is unclear: ask
   the user. Never default to the complex form.

The reason must be recorded: sidecar adapters implement
`FormChoiceReason() string`; embedded callers pass the reason to
`RegisterEmbeddedPlugin`. Both land in `plugin.register.metadata.reason`
for review (Phase-2 risk 6).

## Minimal adapter checklist (Phase 3 fills these)

- Task loop: `Hooks.OnTask` -> work -> `Progress` / `Completed` (report
  must be non-empty when `report.generate` is declared) / `Failed`.
- Approvals: declare the `approval` capability, set ApprovalMode —
  native_forward forwards host approval events; tool_hook intercepts tool
  calls and blocks on `Hooks.OnApproval`.
- Reconnect: one manual retry per user action (`Manager.Retry`); after
  rebind, TASK_LOST_ON_RECONNECT is reported for lost tasks.

## Samples

    cd plugin-sdk
    go run ./cmd/embedded-sample   # registers plugin:sample-embedded:*
    go run ./cmd/sidecar-sample    # registers plugin:sample-sidecar:*
