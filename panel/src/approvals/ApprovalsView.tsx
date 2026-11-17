import { useState } from "react";
import { App as AntdApp, Button, Drawer, Input, Popconfirm, Space, Table, Tabs, Tag, Typography } from "antd";
import { usePanel } from "../shared/store";

const riskColor: Record<string, string> = { high: "red", medium: "orange", low: "blue" };
const statusColor: Record<string, string> = {
  pending: "gold", PENDING: "gold", approved: "green", denied: "red",
  modified: "purple", timeout_denied: "red",
};

export function ApprovalsView() {
  const approvals = usePanel((s) => s.approvals);
  const client = usePanel((s) => s.client);
  const refreshAll = usePanel((s) => s.refreshAll);
  const { message } = AntdApp.useApp();
  const [tab, setTab] = useState("pending");
  const [modifying, setModifying] = useState<string | null>(null);
  const [modified, setModified] = useState("{}");

  const respond = async (approval_id: string, decision: string, modifiedParams?: string) => {
    if (!client) return;
    try {
      const params: Record<string, unknown> = { approval_id, decision };
      if (decision === "modify") {
        try { JSON.parse(modifiedParams ?? "{}"); } catch { message.error("JSON 不合法"); return; }
        params.modified_params = JSON.parse(modifiedParams ?? "{}");
      }
      await client.call("panel.approvals.respond", params);
      message.success("已裁决");
      setModifying(null);
      await refreshAll();
    } catch (e) {
      message.error(String((e as Error).message));
    }
  };

  const pending = approvals.filter((a) => ["pending", "PENDING"].includes(a.status));
  const resolved = approvals.filter((a) => !["pending", "PENDING"].includes(a.status));

  const cols = (isPending: boolean) => [
    { title: "发起实例", dataIndex: "instance_id" },
    { title: "操作类型", dataIndex: "operation_type" },
    {
      title: "风险", dataIndex: "risk_level",
      render: (v: string) => <Tag color={riskColor[v] ?? "default"}>{v}</Tag>,
    },
    { title: "摘要", dataIndex: "context_summary", ellipsis: true },
    { title: "任务", dataIndex: "task_id", ellipsis: true },
    ...(isPending
      ? [{
          title: "裁决", render: (_: unknown, r: ApprovalRow) => (
            <Space>
              <Popconfirm title="批准该操作？" onConfirm={() => respond(r.approval_id, "approve")}>
                <Button type="primary" size="small">批准</Button>
              </Popconfirm>
              <Button size="small" onClick={() => { setModifying(r.approval_id); setModified("{}"); }}>修改</Button>
              <Popconfirm title="拒绝该操作？" onConfirm={() => respond(r.approval_id, "deny")}>
                <Button danger size="small">拒绝</Button>
              </Popconfirm>
            </Space>
          ),
        }]
      : [{
          title: "结果", dataIndex: "status",
          render: (v: string) => <Tag color={statusColor[v] ?? "default"}>{v}</Tag>,
        },
        { title: "理由", dataIndex: "decision", render: (v?: string) => v || "-" }]
  )];

  return (
    <>
      <Tabs
        activeKey={tab} onChange={setTab}
        items={[
          { key: "pending", label: `待决 (${pending.length})`, children: (
            <Table<ApprovalRow> rowKey="approval_id" dataSource={pending} pagination={false} columns={cols(true)} size="middle" />
          ) },
          { key: "resolved", label: `已决 (${resolved.length})`, children: (
            <Table<ApprovalRow> rowKey="approval_id" dataSource={resolved} pagination={{ pageSize: 20 }} columns={cols(false)} size="middle" />
          ) },
        ]}
      />
      <Drawer
        open={!!modifying} onClose={() => setModifying(null)} width={520}
        title={<Typography.Text>修改参数（JSON）</Typography.Text>}
      >
        <Input.TextArea
          rows={14} value={modified}
          onChange={(e) => setModified(e.target.value)}
          style={{ fontFamily: "monospace" }}
        />
        <Space style={{ marginTop: 12 }}>
          <Button type="primary" onClick={() => modifying && respond(modifying, "modify", modified)}>提交</Button>
          <Button onClick={() => setModifying(null)}>取消</Button>
        </Space>
      </Drawer>
    </>
  );
}

type ApprovalRow = NonNullable<ReturnType<typeof usePanel.getState>["approvals"]>[number];
