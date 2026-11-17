import { Table, Tag, Typography } from "antd";
import { usePanel } from "../shared/store";

const connColor: Record<string, string> = { READY: "green", UNREACHABLE: "orange", DISCONNECTED: "red" };
const statusColor: Record<string, string> = { idle: "blue", busy: "gold", waiting_approval: "purple", error: "red" };

export function AgentsView() {
  const agents = usePanel((s) => s.agents);
  return (
    <Table<AgentRow>
      rowKey="instance_id"
      dataSource={agents}
      pagination={false}
      size="middle"
      columns={[
        { title: "实例", dataIndex: "instance_id", render: (v: string) => <Typography.Text copyable>{v}</Typography.Text> },
        { title: "类型", dataIndex: "agent_type" },
        {
          title: "连接", dataIndex: "conn_status",
          render: (v: string) => <Tag color={connColor[v] ?? "default"}>{v}</Tag>,
        },
        {
          title: "状态", dataIndex: "status",
          render: (v: string) => <Tag color={statusColor[v] ?? "default"}>{v}</Tag>,
        },
        { title: "审批模式", dataIndex: "approval_mode", render: (v?: string) => v || "-" },
        { title: "当前任务", dataIndex: "current_task_id", render: (v?: string) => v || "-" },
      ]}
    />
  );
}

type AgentRow = NonNullable<ReturnType<typeof usePanel.getState>["agents"]>[number];
