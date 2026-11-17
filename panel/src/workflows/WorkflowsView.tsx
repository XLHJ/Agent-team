import { App as AntdApp, Button, Empty, Input, Modal, Popconfirm, Space, Table, Tag } from "antd";
import { useState } from "react";
import { usePanel } from "../shared/store";

export function WorkflowsView() {
  const workflows = usePanel((s) => s.workflows);
  const client = usePanel((s) => s.client);
  const refreshAll = usePanel((s) => s.refreshAll);
  const { message } = AntdApp.useApp();
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ name: "", stages: "" });

  const save = async () => {
    if (!client) return;
    // 简单顺序链：每行一个阶段 "instance_id | task_prompt | report_prompt"
    const stages = form.stages.split("\n").filter((l) => l.trim()).map((l, i) => {
      const [agent, taskPrompt, reportPrompt] = l.split("|").map((s) => s.trim());
      return { order_index: i, agent_instance_id: agent, task_prompt: taskPrompt, report_prompt: reportPrompt || "总结完成项/未完成项/产物/风险" };
    });
    if (!form.name || stages.length < 1) {
      message.warning("需要链名称与至少一个阶段");
      return;
    }
    try {
      await client.call("panel.workflows.save", { name: form.name, stages });
      message.success("已保存");
      setCreating(false);
      await refreshAll();
    } catch (e) {
      message.error(String((e as Error).message));
    }
  };

  return (
    <>
      <Space style={{ marginBottom: 12 }}>
        <Button type="primary" onClick={() => setCreating(true)}>新建协作链</Button>
        <Button onClick={() => refreshAll()}>刷新</Button>
      </Space>
      <Table<WorkflowRow>
        rowKey="workflow_id"
        dataSource={workflows}
        pagination={false}
        locale={{ emptyText: <Empty description="暂无协作链" /> }}
        columns={[
          { title: "名称", dataIndex: "name" },
          { title: "状态", dataIndex: "status", render: (v: string) => <Tag color={v === "active" ? "green" : "default"}>{v}</Tag> },
          { title: "阶段数", render: (_: unknown, r: WorkflowRow) => r.stages?.length ?? 0 },
          {
            title: "操作", render: (_: unknown, r: WorkflowRow) => (
              <Space>
                <Button size="small" type="primary" onClick={async () => {
                  await client?.call("panel.workflows.start", { workflow_id: r.workflow_id });
                  message.success("已启动");
                  await refreshAll();
                }}>启动</Button>
                <Popconfirm title="删除该链？" onConfirm={async () => {
                  await client?.call("panel.workflows.delete", { workflow_id: r.workflow_id });
                  await refreshAll();
                }}>
                  <Button size="small" danger>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal
        open={creating} onOk={save} onCancel={() => setCreating(false)} title="新建顺序协作链" width={640}
      >
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <Input placeholder="链名称" value={form.name} onChange={(e: { target: { value: string } }) => setForm({ ...form, name: e.target.value })} />
          <Input.TextArea
            rows={6}
            placeholder={"每行一个阶段：实例ID | 任务提示词 | 报告提示词\n例如：\ncli:fakeagent:xxx | 写一份周报 | 用3句话总结完成情况"}
            value={form.stages}
            onChange={(e: { target: { value: string } }) => setForm({ ...form, stages: e.target.value })}
          />
        </Space>
      </Modal>
    </>
  );
}

type WorkflowRow = NonNullable<ReturnType<typeof usePanel.getState>["workflows"]>[number];
