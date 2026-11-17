// 与 Agent 的对话记录：zustand persist，刷新/重载后保留。
import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface ChatMsg {
  id: string;
  role: "user" | "agent";
  text: string;
  at: number;
  pending?: boolean;
  failed?: boolean;
}

interface ChatState {
  byAgent: Record<string, ChatMsg[]>;
  add: (agent: string, msg: ChatMsg) => void;
  patch: (agent: string, id: string, p: Partial<ChatMsg>) => void;
  clear: (agent: string) => void;
}

export const useChat = create<ChatState>()(
  persist(
    (set) => ({
      byAgent: {},
      add: (agent, msg) =>
        set((s) => ({ byAgent: { ...s.byAgent, [agent]: [...(s.byAgent[agent] ?? []), msg] } })),
      patch: (agent, id, p) =>
        set((s) => ({
          byAgent: {
            ...s.byAgent,
            [agent]: (s.byAgent[agent] ?? []).map((m) => (m.id === id ? { ...m, ...p } : m)),
          },
        })),
      clear: (agent) =>
        set((s) => {
          const next = { ...s.byAgent };
          delete next[agent];
          return { byAgent: next };
        }),
    }),
    { name: "agentpp-chat-v1" }
  )
);

export const chatUid = (): string =>
  typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
