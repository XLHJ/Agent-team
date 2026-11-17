// 首页（监控统计）：统计填充卡（panel.usage.summary 真实汇总）+ Agent 3 列网格。
import { useEffect, useMemo, useState } from "react";
import { usePanel } from "../../shared/store";
import { M3Button, ripple } from "../ui";

export interface UsageRow {
  model: string;
  source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  avg_ttft_ms: number;
  avg_speed_tokens_per_s: number;
}

export interface UsageSummary {
  window: string;
  since_ms: number;
  until_ms: number;
  rows: UsageRow[];
  totals: { calls: number; input_tokens: number; output_tokens: number };
}

// 选取"最近活跃"的 agent：优先有当前任务的，其次最近一条任务所属实例，最后第一个。
export function pickActiveAgent(): string | null {
  const { agents, tasks } = usePanel.getState();
  if (!agents.length) return null;
  const busy = agents.find((a) => a.current_task_id);
  if (busy) return busy.instance_id;
  const lastTask = tasks.find((t) => agents.some((a) => a.instance_id === t.instance_id));
  return lastTask?.instance_id ?? agents[0].instance_id;
}

export function HomeScreen(props: { onOpenAgent: (instanceId: string) => void }) {
  const { onOpenAgent } = props;
  const agents = usePanel((s) => s.agents);
  const client = usePanel((s) => s.client);
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!client) return;
    let alive = true;
    setFailed(false);
    client
      .call<UsageSummary>("panel.usage.summary", { window: "31d" })
      .then((r) => {
        if (alive) setSummary(r);
      })
      .catch(() => {
        if (alive) setFailed(true);
      });
    return () => {
      alive = false;
    };
  }, [client]);

  const totals = summary?.totals;
  const detailTarget = useMemo(() => pickActiveAgent(), [agents]);

  return (
    <>
      <section className="card home-stats">
        <h2 className="card-title">统计</h2>
        <div className="card-body">
          {failed ? (
            <div className="empty-state">
              <span className="msr">bar_chart</span>
              <span>统计数据加载失败，请检查 Hub 连接后重试。</span>
            </div>
          ) : !summary ? (
            <span className="muted">正在加载统计数据……</span>
          ) : (
            <>
              <div className="stat-line">
                <span className="muted">调用次数</span>
                <b>{totals?.calls.toLocaleString() ?? "0"}</b>
              </div>
              <div className="stat-line">
                <span className="muted">输入 Token 总量</span>
                <b>{totals?.input_tokens.toLocaleString() ?? "0"}</b>
              </div>
              <div className="stat-line">
                <span className="muted">输出 Token 总量</span>
                <b>{totals?.output_tokens.toLocaleString() ?? "0"}</b>
              </div>
            </>
          )}
          <M3Button
            variant="tonal"
            icon="arrow_forward"
            disabled={!detailTarget}
            onClick={() => detailTarget && onOpenAgent(detailTarget)}
          >
            进入详细统计
          </M3Button>
        </div>
      </section>

      {agents.length === 0 ? (
        <section className="card agent-grid" style={{ gridTemplateColumns: "1fr" }}>
          <div className="empty-state">
            <span className="msr">groups</span>
            <b>暂无接入的 Agent</b>
            <span className="muted card-body">Agent 注册并连接 Hub 后会显示在这里。</span>
          </div>
        </section>
      ) : (
        <div className="agent-grid">
          {agents.map((a) => (
            <button
              key={a.instance_id}
              type="button"
              className="card m3-ripple"
              onClick={() => onOpenAgent(a.instance_id)}
              onPointerDown={ripple}
            >
              <h3 className="card-title">{a.instance_id}</h3>
              <div className="card-body agent-lines">
                <div>
                  <span className="muted">当前模型：</span>—
                </div>
                <div>
                  <span className="muted">当前状态：</span>
                  {a.status ? <span className="chip">{a.status}</span> : "—"}
                </div>
                <div>
                  <span className="muted">总输入 Token：</span>—
                </div>
                <div>
                  <span className="muted">总输出 Token：</span>—
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </>
  );
}
