// 屏幕9 Agent 详情：名称卡（打开agent→snackbar）+ 统计卡（panel.usage.summary 汇总表格）
// + 信息卡 + 对话框（panel.tasks.create 派发、panel.tasks.get 轮询，消息 zustand persist）。
import { useEffect, useMemo, useRef, useState } from "react";
import { usePanel, type AgentRow } from "../../shared/store";
import { useChat, chatUid } from "../chatStore";
import { M3Button, M3Field } from "../ui";
import type { UsageRow, UsageSummary } from "./HomeScreen";

interface TaskLike {
  task_id: string;
  state: string;
  raw_output?: string;
  report?: string;
}

const TERMINAL = new Set(["completed", "failed", "canceled"]);
const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

interface AggRow {
  model: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  avg_ttft_ms: number;
  avg_speed_tokens_per_s: number;
}

// 按 (model, source) 原始行聚合：按模型汇总；平均指标按调用次数加权。
function aggregate(rows: UsageRow[]): { perModel: AggRow[]; total: AggRow } {
  const byModel = new Map<string, { calls: number; inTok: number; outTok: number; ttftW: number; speedW: number }>();
  const total = { calls: 0, inTok: 0, outTok: 0, ttftW: 0, speedW: 0 };
  for (const r of rows) {
    const cur =
      byModel.get(r.model) ?? { calls: 0, inTok: 0, outTok: 0, ttftW: 0, speedW: 0 };
    cur.calls += r.calls;
    cur.inTok += r.input_tokens;
    cur.outTok += r.output_tokens;
    cur.ttftW += r.avg_ttft_ms * r.calls;
    cur.speedW += r.avg_speed_tokens_per_s * r.calls;
    byModel.set(r.model, cur);
    total.calls += r.calls;
    total.inTok += r.input_tokens;
    total.outTok += r.output_tokens;
    total.ttftW += r.avg_ttft_ms * r.calls;
    total.speedW += r.avg_speed_tokens_per_s * r.calls;
  }
  const mk = (model: string, v: { calls: number; ttftW: number; speedW: number }): AggRow => ({
    model,
    calls: v.calls,
    input_tokens: 0,
    output_tokens: 0,
    avg_ttft_ms: v.calls ? v.ttftW / v.calls : 0,
    avg_speed_tokens_per_s: v.calls ? v.speedW / v.calls : 0,
  });
  const perModel = [...byModel.entries()].map(([model, v]) => {
    const row = mk(model, v);
    row.input_tokens = v.inTok;
    row.output_tokens = v.outTok;
    return row;
  });
  const t = mk("合计", total);
  t.input_tokens = total.inTok;
  t.output_tokens = total.outTok;
  return { perModel, total: t };
}

export function AgentDetailScreen(props: { instanceId: string }) {
  const { instanceId } = props;
  const agent: AgentRow | undefined = usePanel((s) => s.agents.find((a) => a.instance_id === instanceId));
  const client = usePanel((s) => s.client);

  // ---- 用量统计（真实数据，panel.usage.summary）----
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [usageFailed, setUsageFailed] = useState(false);
  useEffect(() => {
    if (!client) return;
    let alive = true;
    setUsageFailed(false);
    client
      .call<UsageSummary>("panel.usage.summary", { window: "31d" })
      .then((r) => {
        if (alive) setSummary(r);
      })
      .catch(() => {
        if (alive) setUsageFailed(true);
      });
    return () => {
      alive = false;
    };
  }, [client]);
  const agg = useMemo(() => (summary ? aggregate(summary.rows) : null), [summary]);

  // ---- 对话 ----
  const msgs = useChat((s) => s.byAgent[instanceId] ?? []);
  const addMsg = useChat((s) => s.add);
  const patchMsg = useChat((s) => s.patch);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [msgs]);

  const canDispatch = !!agent?.capabilities?.includes("task.dispatch");

  const send = async () => {
    const prompt = draft.trim();
    if (!prompt || sending || !client) return;
    setDraft("");
    addMsg(instanceId, { id: chatUid(), role: "user", text: prompt, at: Date.now() });
    const aid = chatUid();
    addMsg(instanceId, {
      id: aid,
      role: "agent",
      text: "任务已提交，等待 Agent 返回……",
      at: Date.now(),
      pending: true,
    });
    setSending(true);
    try {
      // Hub 未暴露独立对话方法；panel.tasks.create 即对话入口（Hub 内部转译 task.dispatch）。
      const res = await client.call<{ task_id: string }>("panel.tasks.create", {
        instance_id: instanceId,
        prompt,
        timeout_ms: 120000,
      });
      const taskId = res.task_id;
      patchMsg(instanceId, aid, { text: `任务 ${taskId} 进行中……` });
      const deadline = Date.now() + 130000;
      let finalText: string | null = null;
      let failed = false;
      while (Date.now() < deadline) {
        await sleep(2000);
        let t: TaskLike | null = null;
        try {
          t = await client.call<TaskLike>("panel.tasks.get", { task_id: taskId });
        } catch {
          continue; // 单次轮询失败不终止等待
        }
        if (t && TERMINAL.has(t.state)) {
          failed = t.state !== "completed";
          finalText =
            (t.raw_output || t.report || "").trim() ||
            (t.state === "completed" ? "任务完成（Agent 未返回文本输出）" : `任务${t.state}`);
          break;
        }
        const progress = t ? (t.report || t.raw_output || "").trim() : "";
        if (progress) patchMsg(instanceId, aid, { text: progress });
      }
      if (finalText === null) {
        failed = true;
        finalText = "等待超时（120s），请稍后在任务看板查看结果。";
      }
      patchMsg(instanceId, aid, { text: finalText, pending: false, failed });
    } catch (e) {
      patchMsg(instanceId, aid, {
        text: `发送失败：${(e as Error).message || String(e)}`,
        pending: false,
        failed: true,
      });
    } finally {
      setSending(false);
    }
  };

  // ---- snackbar（store 无 agent 原界面 URL）----
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(toastTimer.current), []);
  const openAgent = () => {
    setToast("面板中未保存该 Agent 的原界面地址，无法直接打开");
    window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToast(null), 3200);
  };

  return (
    <>
      <section className="card detail-head">
        <div style={{ minWidth: 0 }}>
          <h2 className="card-title">{instanceId}</h2>
          <div className="card-body muted detail-head-meta">{agent?.agent_type ?? "—"}</div>
        </div>
        <M3Button icon="arrow_forward" onClick={openAgent}>
          打开agent
        </M3Button>
      </section>

      <div className="detail-row">
        <section className="card detail-stats">
          <h2 className="card-title">统计</h2>
          <div className="card-body">
            {usageFailed ? (
              <div className="empty-state">
                <span className="msr">bar_chart</span>
                <span>用量数据加载失败，请检查 Hub 连接。</span>
              </div>
            ) : !summary ? (
              <span className="muted">正在加载用量数据……</span>
            ) : summary.rows.length === 0 ? (
              <div className="empty-state">
                <span className="msr">bar_chart</span>
                <span>该时间范围内暂无用量数据</span>
                <span className="muted">Agent 插件需在任务结束时上报 usage.report。</span>
              </div>
            ) : (
              <table className="stats-table">
                <thead>
                  <tr>
                    <th>模型</th>
                    <th className="num">调用次数</th>
                    <th className="num">输入 Token</th>
                    <th className="num">输出 Token</th>
                    <th className="num">平均首字延迟</th>
                    <th className="num">平均生成速度</th>
                  </tr>
                </thead>
                <tbody>
                  {agg!.perModel.map((r) => (
                    <tr key={r.model}>
                      <td>{r.model}</td>
                      <td className="num">{r.calls.toLocaleString()}</td>
                      <td className="num">{r.input_tokens.toLocaleString()}</td>
                      <td className="num">{r.output_tokens.toLocaleString()}</td>
                      <td className="num">{r.avg_ttft_ms.toFixed(0)} ms</td>
                      <td className="num">{r.avg_speed_tokens_per_s.toFixed(1)} tokens/s</td>
                    </tr>
                  ))}
                  <tr className="total">
                    <td>{agg!.total.model}</td>
                    <td className="num">{agg!.total.calls.toLocaleString()}</td>
                    <td className="num">{agg!.total.input_tokens.toLocaleString()}</td>
                    <td className="num">{agg!.total.output_tokens.toLocaleString()}</td>
                    <td className="num">{agg!.total.avg_ttft_ms.toFixed(0)} ms</td>
                    <td className="num">{agg!.total.avg_speed_tokens_per_s.toFixed(1)} tokens/s</td>
                  </tr>
                </tbody>
              </table>
            )}
          </div>
        </section>

        <section className="card detail-info">
          <h2 className="card-title">{instanceId}</h2>
          <div className="card-body agent-lines">
            <div>
              <span className="muted">模型：</span>—
            </div>
            <div>
              <span className="muted">状态：</span>
              {agent?.status ? <span className="chip">{agent.status}</span> : "—"}
              <span className="muted">　Token：</span>—
            </div>
          </div>
        </section>
      </div>

      <section className="card detail-chat">
        <h2 className="card-title">对话框</h2>
        <div className="chat-list" ref={listRef}>
          {msgs.length === 0 ? (
            <div className="empty-state muted" style={{ margin: "auto 0" }}>
              <span className="msr">forum</span>
              <span>发送消息即向该 Agent 派发一个任务（120s 超时）。</span>
              {!canDispatch && (
                <span className="muted">注意：该 Agent 未声明 task.dispatch 能力，派发会被 Hub 拒绝。</span>
              )}
            </div>
          ) : (
            msgs.map((m) => (
              <div key={m.id} className={`bubble ${m.role}${m.failed ? " failed" : ""}`}>
                {m.text}
              </div>
            ))
          )}
        </div>
        {sending && (
          <div className="progress indeterminate chat-progress">
            <i />
          </div>
        )}
        <div className="chat-input-row">
          <M3Field
            label="输入消息"
            value={draft}
            onChange={setDraft}
            onEnter={() => void send()}
          />
          <button
            type="button"
            className="chat-send m3-ripple"
            aria-label="发送"
            disabled={sending || !draft.trim() || !client}
            onClick={() => void send()}
          >
            <span className="msr">send</span>
          </button>
        </div>
      </section>

      {toast && <div className="snackbar">{toast}</div>}
    </>
  );
}
