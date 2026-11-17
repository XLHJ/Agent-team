// Panel channel frames. Panel methods are separate from the plugin protocol;
// they share JSON-RPC 2.0 framing only.
export type PanelMethod =
  | "panel.subscribe"
  | "panel.unsubscribe"
  | "panel.agents.list"
  | "panel.tasks.list"
  | "panel.tasks.get"
  | "panel.tasks.create"
  | "panel.tasks.cancel"
  | "panel.approvals.list"
  | "panel.approvals.respond"
  | "panel.intake.candidates"
  | "panel.intake.confirm"
  | "panel.intake.ignore"
  | "panel.intake.rescan"
  | "panel.workflows.list"
  | "panel.workflows.get"
  | "panel.workflows.save"
  | "panel.workflows.start"
  | "panel.workflows.runs"
  | "panel.workflows.run"
  | "panel.workflows.cancel"
  | "panel.workflows.delete"
  | "panel.usage.summary"
  | "panel.usage.logs"
  | "panel.kitesurf.get"
  | "panel.kitesurf.set";

export interface PanelRequest {
  jsonrpc: "2.0";
  id: number;
  method: PanelMethod;
  params: Record<string, unknown>;
}

export interface PanelEventFrame {
  jsonrpc: "2.0";
  method: "panel.event";
  params: { event_type: string; body: Record<string, unknown> };
}

export type ServerFrame =
  | { jsonrpc: "2.0"; id: number; result?: unknown; error?: PanelError }
  | PanelEventFrame;

export interface PanelError {
  code: number;
  message: string;
  data?: { code?: string; field?: string };
}

export const bizCode = (e: PanelError | undefined): string => e?.data?.code ?? e?.message ?? "";
