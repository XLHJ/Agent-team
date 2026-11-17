import { useState } from "react";
import { App as AntdApp, Button, Descriptions, Drawer, Input, Select, Space, Table, Tag, Typography } from "antd";
import { usePanel } from "../shared/store";

const stateColor: Record<string, string> = {
  dispatched: "gold", running: "blue", waiting_approval: "purple",
  completed: "green", failed: "red", canceled: "default",
};

export function TasksView() {
  const tasks = usePanel((s) => s.tasks);
  const agents = usePanel((s) => s.agents);
  const client = usePanel((s) => s.client);
  const refreshAll = usePanel((s) => s.refreshAll);
  const { message } = AntdApp.useApp();
  const [filterState, setFilterState] = useState<string | undefined>();
  const [filterAgent, setFilterAgent] = useState<string | undefined>();
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ instance_id: "", prompt: "", report_prompt: "", timeout_ms: 60000 });

  const rows = tasks.filter(
    (t) => (!filterState || t.state === filterState) && (!filterAgent || t.instance_id === filterAgent)
  );
  const detail = tasks.find((t) => t.task_id === selected);
  const dispatchable = agents.filter((a) => a.conn_status === "READY" && a.capabilities?.includes("task.dispatch"));

  const create = async () => {
    if (!client || !form.instance_id || !form.prompt) {
      message.warning("需要选择实例并填写提示词");
      return;
    }
    try {
      await client.call("panel.tasks.create", form);
      message.success("已派发");
      setCreating(false);
      setForm({ instance_id: "", prompt: "", report_prompt: "", timeout_ms: 60000 });
      await refreshAll();
    } catch (e) {
      message.error(String((e as Error).message));
    }
  };

  return (
    <>
      <Space style={{ marginBottom: 12 }} wrap>
        <Select
          allowClear placeholder="状态" style={{ width: 160 }} value={filterState}
          onChange={setFilterState}
          options={Object.keys(stateColor).map((k) => ({ value: k, label: k }))}
        />
        <Select
          allowClear placeholder="实例" style={{ width: 220 }} value={filterAgent}
          onChange={setFilterAgent}
          options={agents.map((a) => ({ value: a.instance_id, label: a.instance_id }))}
        />
        <Button type="primary" onClick={() => setCreating(true)}>创建任务</Button>
      </Space>
      <Table<TaskRow>
        rowKey="task_id"
        dataSource={rows}
        size="middle"
        pagination={{ pageSize: 20 }}
        onRow={(r) => ({ onClick: () => setSelected(r.task_id), style: { cursor: "pointer" } })}
        columns={[
          { title: "任务", dataIndex: "task_id", render: (v: string) => <Typography.Text copyable>{v}</Typography.Text> },
          { title: "实例", dataIndex: "instance_id" },
          {
            title: "状态", dataIndex: "state",
            render: (v: string) => <Tag color={stateColor[v] ?? "default"}>{v}</Tag>,
          },
          { title: "提示词", dataIndex: "prompt", ellipsis: true },
        ]}
      />
      <Drawer
        open={!!detail} onClose={() => setSelected(null)} width={560}
        title={detail ? `任务 ${detail.task_id}` : ""}
      >
        {detail && (
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="状态">
              <Tag color={stateColor[detail.state] ?? "default"}>{detail.state}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="实例">{detail.instance_id}</Descriptions.Item>
            <Descriptions.Item label="提示词">{detail.prompt}</Descriptions.Item>
            {detail.workflow_run_id && (
              <Descriptions.Item label="所属协作链">
                {detail.workflow_run_id}（阶段 {detail.stage_index}）
              </Descriptions.Item>
            )}
            {detail.raw_output && <Descriptions.Item label="原始输出">{detail.raw_output}</Descriptions.Item>}
            {detail.report && <Descriptions.Item label="进度报告">{detail.report}</Descriptions.Item>}
          </Descriptions>
        )}
        {detail && ["dispatched", "running", "waiting_approval"].includes(detail.state) && (
          <Button
            danger style={{ marginTop: 12 }}
            onClick={async () => {
              await client?.call("panel.tasks.cancel", { task_id: detail.task_id });
              await refreshAll();
            }}
          >
            取消任务
          </Button>
        )}
      </Drawer>
      <Drawer open={creating} onClose={() => setCreating(false)} title="创建任务" width={480}>
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <Select
            placeholder="目标实例（仅 READY 且声明 task.dispatch）" style={{ width: "100%" }}
            value={form.instance_id || undefined}
            onChange={(v) => setForm({ ...form, instance_id: v })}
            options={dispatchable.map((a) => ({ value: a.instance_id, label: a.instance_id }))}
          />
          <Input.TextArea
            rows={4} placeholder="任务提示词（必填）"
            value={form.prompt} onChange={(e) => setForm({ ...form, prompt: e.target.value })}
          />
          <Input.TextArea
            rows={2} placeholder="报告提示词（可选；协作链阶段会单独配置）"
            value={form.report_prompt} onChange={(e) => setForm({ ...form, report_prompt: e.target.value })}
          />
          <Button type="primary" block onClick={create}>派发</Button>
        </Space>
      </Drawer>
    </>
  );
}

type TaskRow = NonNullable<ReturnType<typeof usePanel.getState>["tasks"]>[number];
