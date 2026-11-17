import { useMemo, useState } from "react";
import { Input, Select, Space, Table, Tag, Typography } from "antd";
import { usePanel } from "../shared/store";

export function EventsView() {
  const events = usePanel((s) => s.events);
  const [typeFilter, setTypeFilter] = useState<string | undefined>();
  const [textFilter, setTextFilter] = useState("");

  const rows = useMemo(
    () =>
      events.filter(
        (e) =>
          (!typeFilter || e.event_type === typeFilter) &&
          (!textFilter || JSON.stringify(e.body).toLowerCase().includes(textFilter.toLowerCase()))
      ),
    [events, typeFilter, textFilter]
  );

  return (
    <>
      <Space style={{ marginBottom: 12 }} wrap>
        <Select
          allowClear placeholder="事件类型" style={{ width: 220 }} value={typeFilter}
          onChange={setTypeFilter}
          options={[...new Set(events.map((e) => e.event_type))].map((t) => ({ value: t, label: t }))}
        />
        <Input.Search
          allowClear placeholder="正文搜索" style={{ width: 260 }}
          onSearch={setTextFilter}
        />
        <Typography.Text type="secondary">仅本次会话内回看，缓冲上限 1000 条</Typography.Text>
      </Space>
      <Table<EventRow>
        rowKey="seq"
        dataSource={rows}
        pagination={{ pageSize: 30 }}
        size="small"
        columns={[
          { title: "时间", dataIndex: "at", width: 100, render: (v: number) => new Date(v).toLocaleTimeString() },
          { title: "类型", dataIndex: "event_type", render: (v: string) => <Tag>{v}</Tag> },
          {
            title: "正文", dataIndex: "body", ellipsis: true,
            render: (v: Record<string, unknown>) => (
              <Typography.Text style={{ fontFamily: "monospace", fontSize: 12 }}>
                {JSON.stringify(v).slice(0, 200)}
              </Typography.Text>
            ),
          },
        ]}
      />
    </>
  );
}

type EventRow = NonNullable<ReturnType<typeof usePanel.getState>["events"]>[number];
