import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Input, Space, Tag, Typography, message } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { usePanel } from "../shared/store";
import { hubEndpoint } from "../api/hub";

interface Status {
  configured: boolean;
  endpoint?: string;
  reachable?: boolean;
  error?: string;
}

// Kitesurf（https://github.com/cloudflare/kitesurf）集成视图。
// Kitesurf 是 Cloudflare 开源的 Workers 原生浏览器引擎（HTML/CSS/JS 渲染 + 截图，
// 无需 Chrome），对外暴露 CDP WebSocket 端点。用户自行运行 kitesurf（wrangler dev
// 或部署到 Workers），在面板配置其浏览器 WS 端点后，Hub 侧桥接 API 让所有接入面板
// 的 agent（以及 Dify 工作流）都能渲染网页。
//
// 版权合规：Apache-2.0（Copyright (c) 2026 Cloudflare, Inc.）+ NOTICE 第三方声明。
// 本集成不 vendor、不修改其源码，仅调用其公开接口。
export function KitesurfView() {
  const client = usePanel((s) => s.client);
  const [endpoint, setEndpoint] = useState("");
  const [draft, setDraft] = useState("");
  const [status, setStatus] = useState<Status | null>(null);
  const [checking, setChecking] = useState(false);

  const load = useCallback(async () => {
    if (!client) return;
    try {
      const res = (await client.call("panel.kitesurf.get", {})) as unknown as {
        endpoint: string;
      };
      setEndpoint(res.endpoint ?? "");
      setDraft(res.endpoint ?? "");
    } catch (e) {
      message.error(`读取配置失败: ${String(e)}`);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  const checkStatus = useCallback(async () => {
    if (!client) return;
    setChecking(true);
    try {
      // Derive the hub HTTP base from the WS endpoint (works in Electron too).
      const ep = await hubEndpoint();
      const u = new URL(ep.url.replace(/^ws/, "http"));
      const httpBase = `${u.protocol}//${u.host}`;
      const token = new URLSearchParams(location.search).get("token") ?? "";
      const res = await fetch(`${httpBase}/kitesurf/v1/status`, {
        headers: { "X-Hub-Token": token },
      });
      setStatus((await res.json()) as unknown as Status);
    } catch (e) {
      message.error(`状态查询失败: ${String(e)}`);
    } finally {
      setChecking(false);
    }
  }, [client]);

  const save = useCallback(async () => {
    if (!client) return;
    let v = draft.trim();
    if (v && !/^wss?:\/\//.test(v)) v = `ws://${v}`;
    try {
      await client.call("panel.kitesurf.set", { endpoint: v });
      message.success("已保存");
      setEndpoint(v);
    } catch (e) {
      message.error(`保存失败: ${String(e)}`);
    }
  }, [client, draft]);

  return (
    <div style={{ maxWidth: 760 }}>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        Kitesurf 无头浏览器
      </Typography.Title>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="让所有接入面板的 Agent 具备网页渲染能力"
        description={
          <>
            Kitesurf 是 Cloudflare 开源的浏览器引擎（Workers 原生，无需 Chrome），请先自行运行：
            <pre style={{ fontSize: 12, background: "#f5f5f5", padding: 8, borderRadius: 4, margin: "8px 0" }}>
{`git clone https://github.com/cloudflare/kitesurf && cd kitesurf
pnpm install && pnpm build
pnpm wrangler dev   # 或部署到 Cloudflare Workers（需 Workers Paid）`}
            </pre>
            然后在下方填入其 CDP 端点（默认 ws://127.0.0.1:8787/devtools/browser）。
            配置存于 Hub（所有 agent 共享），之后任意 agent 调用：
          </>
        }
      />
      <Space.Compact style={{ width: "100%" }}>
        <Input
          prefix={<ApiOutlined />}
          placeholder="ws://127.0.0.1:8787/devtools/browser"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onPressEnter={save}
        />
        <Button type="primary" onClick={save}>保存</Button>
        <Button onClick={checkStatus} loading={checking}>检测状态</Button>
      </Space.Compact>
      <Space style={{ margin: "12px 0" }} wrap>
        <Tag color={endpoint ? "green" : "default"}>{endpoint ? "已配置" : "未配置"}</Tag>
        {endpoint && <Typography.Text code style={{ fontSize: 12 }}>{endpoint}</Typography.Text>}
        {status?.configured && (
          <Tag color={status.reachable ? "green" : "red"}>
            {status.reachable ? "Kitesurf 在线" : "Kitesurf 不可达"}
          </Tag>
        )}
      </Space>
      <pre
        style={{
          fontSize: 12, background: "#f6f8fa", padding: 12, borderRadius: 8,
          overflow: "auto", margin: 0,
        }}
      >
{`GET  {HUB}/kitesurf/v1/status                 配置状态 + /health 探测
POST {HUB}/kitesurf/v1/render                 渲染网页，返回 {title, text, screenshot_base64}
     -H "X-Hub-Token: <token>" -d '{"url":"https://example.com","screenshot":true}'`}
      </pre>
      <Typography.Paragraph type="secondary" style={{ marginTop: 12, fontSize: 12 }}>
        许可说明：Kitesurf 采用 Apache-2.0（Copyright (c) 2026 Cloudflare, Inc.）并附带
        NOTICE 第三方组件声明。本面板不 vendor、不修改其源码，仅调用其公开的 CDP/HTTP
        接口；如需分发其代码请保留 LICENSE 与 NOTICE。
      </Typography.Paragraph>
    </div>
  );
}
