import { Typography } from "antd";

// MCP Docs（universal-docs）子应用，通过 iframe 嵌入。
// 子应用源码见 panel/mcp-docs-app/（MIT, Copyright (c) 2026 Scott Persinger）。
// 完整文档编辑功能（Excel/Word/PPT/PDF/图片）需要其 Node 服务端（panel/mcp-docs-app 下
// npm run dev:server，端口 3001）；后端不可用时降级为产品落地页/登录页。
export function McpDocsView() {
  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        文档编辑器
      </Typography.Title>
      <iframe
        src="mcp-docs-app/"
        title="MCP Docs"
        style={{
          width: "100%",
          height: "calc(100vh - 64px - 32px - 46px)",
          minHeight: 480,
          border: 0,
          borderRadius: 8,
          background: "#ffffff",
        }}
      />
    </div>
  );
}
