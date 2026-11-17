import { Typography } from "antd";

// ApexForge (AI 3D CAD 工作台) 子应用，通过 iframe 嵌入。
// 子应用源码见 panel/apexforge-app/；许可证为 GPL-3.0（见其 LICENSE 与 README）。
// 前端在后端 API 不可用时会自动回退到本地模板生成，核心 3D 预览/编辑功能仍可用。
export function ApexForgeView() {
  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        AI 3D 建模
      </Typography.Title>
      <iframe
        src="apexforge-app/"
        title="ApexForge Studio"
        style={{
          width: "100%",
          height: "calc(100vh - 64px - 32px - 46px)",
          minHeight: 480,
          border: 0,
          borderRadius: 8,
          background: "#0a0a0a",
        }}
      />
    </div>
  );
}
