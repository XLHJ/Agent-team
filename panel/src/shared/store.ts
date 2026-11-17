// Zustand store: Hub-authoritative state only. No optimistic updates —
// every mutation goes through the Hub and lands via panel.event (Phase-3 5.2).
import { create } from "zustand";
import type { PanelClient } from "../api/client";

export interface AgentRow {
  instance_id: string;
  agent_type: string;
  conn_status: string;
  status: string;
  current_task_id?: string;
  approval_mode?: string;
  capabilities?: string[];
}

export interface TaskRow {
  task_id: string;
  instance_id: string;
  state: string;
  prompt: string;
  raw_output?: string;
  report?: string;
  workflow_run_id?: string;
  stage_index?: number;
  created_at?: string;
}

export interface ApprovalRow {
  approval_id: string;
  task_id: string;
  instance_id: string;
  operation_type: string;
  risk_level: string;
  context_summary: string;
  status: string;
  decision?: string;
  created_at?: string;
  resolved_at?: string;
}

export interface EventRow {
  seq: number;
  event_type: string;
  body: Record<string, unknown>;
  at: number;
}

export interface WorkflowRow {
  workflow_id: string;
  name: string;
  status: string;
  stages: { stage_id: string; order_index: number; agent: string; task_prompt: string }[];
  updated_at?: number;
}

export interface IntakeRow {
  signature: string;
  display_name: string;
  version: string;
  methods: string[];
  state: string;
}

interface PanelState {
  connected: boolean;
  agents: AgentRow[];
  tasks: TaskRow[];
  approvals: ApprovalRow[];
  events: EventRow[];
  workflows: WorkflowRow[];
  intake: IntakeRow[];
  eventSeq: number;
  client: PanelClient | null;
  setConnected: (v: boolean) => void;
  setClient: (c: PanelClient | null) => void;
  refreshAll: () => Promise<void>;
  applyEvent: (eventType: string, body: Record<string, unknown>) => void;
  pushEvent: (eventType: string, body: Record<string, unknown>) => void;
}

export const EVENT_BUFFER_CAP = 1000;

export const usePanel = create<PanelState>((set, get) => ({
  connected: false,
  agents: [],
  tasks: [],
  approvals: [],
  events: [],
  workflows: [],
  intake: [],
  eventSeq: 0,
  client: null,
  setConnected: (v) => set({ connected: v }),
  setClient: (c) => set({ client: c }),
  refreshAll: async () => {
    const c = get().client;
    if (!c) return;
    try {
      const [a, t, ap, wf, ik] = await Promise.all([
        c.call<{ instances: AgentRow[] }>("panel.agents.list"),
        c.call<{ tasks: TaskRow[] }>("panel.tasks.list", { limit: 200 }),
        c.call<{ items: ApprovalRow[] }>("panel.approvals.list", { status: "all", limit: 200 }),
        c.call<{ workflows: WorkflowRow[] }>("panel.workflows.list"),
        c.call<{ candidates: IntakeRow[] }>("panel.intake.candidates"),
      ]);
      set({
        agents: a.instances ?? [],
        tasks: t.tasks ?? [],
        approvals: ap.items ?? [],
        workflows: wf.workflows ?? [],
        intake: ik.candidates ?? [],
      });
    } catch {
      /* surfaced by the connection banner */
    }
  },
  applyEvent: (eventType, body) => {
    get().pushEvent(eventType, body);
    const s = stringify(body);
    set((st) => {
      switch (eventType) {
        case "agent.status":
        case "agent.registered":
        case "agent.disconnected": {
          const agents = st.agents.map((a) =>
            a.instance_id === body.instance_id ? { ...a, ...body } : a
          );
          return { agents };
        }
        case "task.created":
        case "task.dispatch":
        case "task.progress":
        case "task.completed":
        case "task.failed":
        case "task.canceled": {
          const task = body as unknown as TaskRow;
          const exists = st.tasks.some((t) => t.task_id === task.task_id);
          const tasks = exists
            ? st.tasks.map((t) => (t.task_id === task.task_id ? { ...t, ...task } : t))
            : [task, ...st.tasks];
          return { tasks };
        }
        case "approval.requested": {
          const a = body as unknown as ApprovalRow;
          return { approvals: [a, ...st.approvals] };
        }
        case "approval.resolved":
        case "approval.timeout": {
          const approvals = st.approvals.map((x) =>
            x.approval_id === body.approval_id ? { ...x, ...body } : x
          );
          return { approvals };
        }
        default:
          return {};
      }
    });
    void s;
  },
  pushEvent: (eventType, body) =>
    set((st) => {
      const seq = st.eventSeq + 1;
      const events = [{ seq, event_type: eventType, body, at: Date.now() }, ...st.events];
      if (events.length > EVENT_BUFFER_CAP) events.length = EVENT_BUFFER_CAP;
      return { events, eventSeq: seq };
    }),
}));

const stringify = (v: unknown) => JSON.stringify(v);
void stringify;
