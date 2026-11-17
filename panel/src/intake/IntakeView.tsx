import { Button, Table, Tag, Typography, App as AntdApp } from "antd";
import { usePanel } from "../shared/store";

export function IntakeView() {
  const intake = usePanel((s) => s.intake);
  const client = usePanel((s) => s.client);
  const refreshAll = usePanel((s) => s.refreshAll);
  const { message } = AntdApp.useApp();

  const act = async (signature: string, method: "panel.intake.confirm" | "panel.intake.ignore") => {
    if (!client) return;
    try {
      await client.call(method === "panel.intake.confirm" ? "panel.intake.confirm" : "panel.intake.ignore", { name: signature });
      message.success(method === "panel.intake.confirm" ? "已接管" : "已忽略");
      await refreshAll();
    } catch (e) {
      message.error(String((e as Error).message));
    }
  };

  return (
    <>
      <div style={{ marginBottom: 12, display: "flex", gap: 8 }}>
        <Button
          onClick={async () => {
            await client?.call("panel.intake.rescan");
            await refreshAll();
            message.success("已重新扫描");
          }}
        >
          重新扫描
        </Button>
        <Typography.Text type="secondary">检测到本机 CLI 后需在此确认接管；忽略只影响本次 Hub 运行。</Typography.Text>
      </div>
      <Table<IntakeRow>
        rowKey="signature"
        dataSource={intake}
        pagination={false}
        columns={[
          { title: "CLI", dataIndex: "display_name" },
          { title: "签名", dataIndex: "signature" },
          { title: "版本", dataIndex: "version", render: (v?: string) => v || "-" },
          {
            title: "接管方式", dataIndex: "methods",
            render: (v: string[]) => v?.map((m) => <Tag key={m}>{m}</Tag>),
          },
          {
            title: "状态", dataIndex: "state",
            render: (v: string) => (
              <Tag color={v === "confirmed" ? "green" : v === "ignored" ? "default" : v === "failed" ? "red" : "gold"}>
                {v}
              </Tag>
            ),
          },
          {
            title: "操作", render: (_, r) =>
              r.state === "pending" ? (
                <Button.Group>
                  <Button type="primary" size="small" onClick={() => act(r.signature, "panel.intake.confirm")}>接管</Button>
                  <Button size="small" onClick={() => act(r.signature, "panel.intake.ignore")}>忽略</Button>
                </Button.Group>
              ) : (
                "-"
              ),
          },
        ]}
      />
    </>
  );
}

type IntakeRow = NonNullable<ReturnType<typeof usePanel.getState>["intake"]>[number];
