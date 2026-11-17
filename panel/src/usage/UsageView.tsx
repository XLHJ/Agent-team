import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Radio, Select, Space, Statistic, Table, Tag, Tooltip, Typography, message } from "antd";
import { DownloadOutlined, ReloadOutlined } from "@ant-design/icons";
import { usePanel } from "../shared/store";

interface UsageRow {
  model: string;
  source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  avg_ttft_ms: number;
  avg_speed_tokens_per_s: number;
}

interface Summary {
  window: string;
  since_ms: number;
  until_ms: number;
  rows: UsageRow[];
  totals: { calls: number; input_tokens: number; output_tokens: number };
}

// 统计看板：多 Agent / 多模型在不同来源下的 Token 用量与性能观测。
// 仅做数据展示与 CSV 导出，不内置价格字典、不计算费用。
export function UsageView() {
  const client = usePanel((s) => s.client);
  const [window, setWindow] = useState<"24h" | "7d" | "31d">("24h");
  const [groupBy, setGroupBy] = useState<"model" | "source">("model");
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchSummary = useCallback(async () => {
    if (!client) return;
    setLoading(true);
    try {
      const res = await client.call("panel.usage.summary", { window });
      setSummary(res as unknown as Summary);
    } catch (e) {
      message.error(`加载统计数据失败: ${String(e)}`);
    } finally {
      setLoading(false);
    }
  }, [client, window]);

  useEffect(() => {
    void fetchSummary();
  }, [fetchSummary]);

  // 前端分组：按“模型”或“来源”聚合展示。
  const rows = useMemo(() => {
    if (!summary) return [];
    const map = new Map<string, UsageRow>();
    for (const r of summary.rows) {
      const key = groupBy === "model" ? r.model : r.source;
      const cur = map.get(key) ?? {
        model: key, source: key, calls: 0, input_tokens: 0,
        output_tokens: 0, avg_ttft_ms: 0, avg_speed_tokens_per_s: 0,
      };
      cur.calls += r.calls;
      cur.input_tokens += r.input_tokens;
      cur.output_tokens += r.output_tokens;
      map.set(key, cur);
    }
    return [...map.values()];
  }, [summary, groupBy]);

  const exportCsv = useCallback(() => {
    if (!summary) return;
    const head = "model,source,calls,input_tokens,output_tokens,avg_ttft_ms,avg_speed_tokens_per_s";
    const lines = summary.rows.map((r) =>
      [r.model, r.source, r.calls, r.input_tokens, r.output_tokens,
        r.avg_ttft_ms.toFixed(1), r.avg_speed_tokens_per_s.toFixed(2)]
        .map((v) => `"${String(v).replaceAll('"', '""')}"`).join(",")
    );
    const blob = new Blob(["\uFEFF" + head + "\n" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `usage-${summary.window}-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [summary]);

  const grouped = groupBy === "model";

  return (
    <div>
      <Space style={{ marginBottom: 12, width: "100%", justifyContent: "space-between" }} wrap>
        <Space wrap>
          <Radio.Group
            optionType="button"
            value={window}
            onChange={(e) => setWindow(e.target.value)}
            options={[
              { value: "24h", label: "24小时" },
              { value: "7d", label: "7天" },
              { value: "31d", label: "31天" },
            ]}
          />
          <Select
            style={{ width: 140 }}
            value={groupBy}
            onChange={(v) => setGroupBy(v)}
            options={[
              { value: "model", label: "按模型分组" },
              { value: "source", label: "按来源分组" },
            ]}
          />
        </Space>
        <Space>
          <Tooltip title="导出 CSV（费用请结合自己的价格表自行计算）">
            <Button icon={<DownloadOutlined />} onClick={exportCsv}>导出 CSV</Button>
          </Tooltip>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchSummary}>刷新</Button>
        </Space>
      </Space>
      {summary && (
        <Space size={32} style={{ marginBottom: 12 }} wrap>
          <Statistic title="调用次数" value={summary.totals.calls} />
          <Statistic title="输入 Token" value={summary.totals.input_tokens} />
          <Statistic title="输出 Token" value={summary.totals.output_tokens} />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            明细保留 31 天，聚合数据长期保留；本看板仅统计用量与速度，不涉及计费。
          </Typography.Text>
        </Space>
      )}
      <Table<UsageRow>
        rowKey={(r) => r.model + "/" + r.source}
        dataSource={rows}
        loading={loading}
        size="small"
        pagination={false}
        columns={[
          {
            title: grouped ? "模型" : "来源",
            dataIndex: grouped ? "model" : "source",
            render: (v: string) => <Tag color="blue">{v}</Tag>,
          },
          { title: "调用次数", dataIndex: "calls", width: 110 },
          { title: "输入 Token", dataIndex: "input_tokens", width: 130 },
          { title: "输出 Token", dataIndex: "output_tokens", width: 130 },
          {
            title: "平均首字延迟 (TTFT)", dataIndex: "avg_ttft_ms", width: 170,
            render: (v: number) => `${v.toFixed(0)} ms`,
          },
          {
            title: "平均生成速度", dataIndex: "avg_speed_tokens_per_s", width: 140,
            render: (v: number) => `${v.toFixed(1)} tokens/s`,
          },
        ]}
      />
      {summary && summary.rows.length === 0 && !loading && (
        <Typography.Text type="secondary">
          该时间范围内暂无用量数据（插件需在任务结束时上报 usage.report）。
        </Typography.Text>
      )}
    </div>
  );
}
