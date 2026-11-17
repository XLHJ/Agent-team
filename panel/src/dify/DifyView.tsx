import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Input, Space, Typography, message } from "antd";
import { GlobalOutlined } from "@ant-design/icons";

const DIFY_URL_KEY = "agentpp.dify.url";

// Dify（https://github.com/langgenius/dify）集成视图。
// Dify 是一个需要独立部署的 LLM 应用平台（Python API + Next.js 前端 + Postgres/Redis/
// 向量库等基础设施），无法像其他子应用一样静态构建内嵌。本视图连接用户自托管的 Dify
// 实例，通过 iframe 嵌入其控制台。
//
// 版权合规：Dify 采用修改版 Apache 2.0（Dify Open Source License）：
//  - 使用其前端时不得移除/修改 LOGO 与版权信息（本集成原样嵌入，不做任何改动）；
//  - 不得用其源码运营多租户 SaaS 服务（本集成不分发、不修改 Dify 源码，仅作客户端入口）。
export function DifyView() {
  const [connected, setConnected] = useState<string | null>(null);
  const [draft, setDraft] = useState("");

  useEffect(() => {
    const saved = localStorage.getItem(DIFY_URL_KEY);
    if (saved) {
      setConnected(saved);
      setDraft(saved);
    }
  }, []);

  const save = useCallback(() => {
    let v = draft.trim();
    if (!v) {
      message.warning("请输入 Dify 实例地址");
      return;
    }
    if (!/^https?:\/\//.test(v)) v = `http://${v}`;
    try {
      // 仅校验格式
      new URL(v);
    } catch {
      message.error("地址格式不正确");
      return;
    }
    localStorage.setItem(DIFY_URL_KEY, v);
    setConnected(v);
    message.success("已保存，正在加载 Dify 控制台…");
  }, [draft]);

  const reset = useCallback(() => {
    localStorage.removeItem(DIFY_URL_KEY);
    setConnected(null);
    setDraft("");
  }, []);

  if (connected) {
    return (
      <div style={{ height: "100%", display: "flex", flexDirection: "column" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
          <Typography.Title level={4} style={{ margin: 0 }}>
            Dify
          </Typography.Title>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {connected}
          </Typography.Text>
          <Button size="small" onClick={reset} style={{ marginLeft: "auto" }}>
            更换实例
          </Button>
        </div>
        <iframe
          src={connected}
          title="Dify 控制台"
          style={{
            width: "100%",
            flex: 1,
            minHeight: 480,
            border: 0,
            borderRadius: 8,
            background: "#f2f5f6",
          }}
        />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        Dify
      </Typography.Title>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="连接你的 Dify 实例"
        description={
          <>
            Dify 是独立部署的 LLM 应用开发平台（含 Python API、Next.js 前端与
            Postgres/Redis/向量库等），需要先用官方 Docker 方式部署。部署完成后，
            在下方填入实例地址（如 <code>http://192.168.1.10:80</code>），面板会以
            iframe 原样嵌入其控制台（保留原始 LOGO 与版权信息）。
          </>
        }
      />
      <Space.Compact style={{ width: "100%" }}>
        <Input
          prefix={<GlobalOutlined />}
          placeholder="Dify 实例地址，例如 http://192.168.1.10:80"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onPressEnter={save}
        />
        <Button type="primary" onClick={save}>
          连接
        </Button>
      </Space.Compact>
      <Typography.Paragraph type="secondary" style={{ marginTop: 16, fontSize: 12 }}>
        部署参考（官方方式）：
        <pre style={{ fontSize: 12, background: "#f5f5f5", padding: 8, borderRadius: 4 }}>
{`git clone https://github.com/langgenius/dify.git
cd dify/docker
cp .env.example .env
docker compose up -d`}
        </pre>
        默认控制台地址为 http://localhost:80（可按 .env 中 EXPOSE_NGINX_PORT 修改）。
      </Typography.Paragraph>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 4 }}>
        <b>与面板 Agent 打通</b>：Dify 工作流可通过 HTTP 节点/自定义工具，把任务派给面板接入的
        Agent。Hub 已暴露桥接 API（鉴权同 Hub bootstrap token，请求头 X-Hub-Token）；
        OpenAPI 工具 Schema 见仓库 docs/dify-bridge.openapi.json，可直接导入 Dify 自定义工具。
      </Typography.Paragraph>
      <pre
        style={{
          fontSize: 12, background: "#f5f5f5", padding: 8, borderRadius: 4,
          overflow: "auto", margin: 0,
        }}
      >
{`GET  {HUB}/bridge/v1/agents            列出可接收任务的 Agent
POST {HUB}/bridge/v1/dispatch          派任务（wait:false 异步 / wait:true 同步等结果）
GET  {HUB}/bridge/v1/tasks/{task_id}   查询任务状态与结果（异步轮询）`}
      </pre>
      <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
        许可说明：Dify 采用修改版 Apache 2.0（Dify Open Source License）。本面板不分发、
        不修改 Dify 源码，仅作为客户端入口连接你自己部署的实例；嵌入其前端时原样保留
        LOGO 与版权信息。请注意其许可禁止用 Dify 源码对外运营多租户 SaaS 服务。
      </Typography.Paragraph>
    </div>
  );
}
