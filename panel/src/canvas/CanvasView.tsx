import { Typography } from "antd";

// 无限画布 (infinite-canvas) 子应用，通过 iframe 嵌入。
// 子应用源码与版权信息见 panel/canvas-app/（MIT, Copyright (c) 2026 basketikun）。
export function CanvasView() {
  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        无限画布
      </Typography.Title>
      <iframe
        src="canvas-app/#/canvas"
        title="无限画布"
        style={{
          width: "100%",
          height: "calc(100vh - 64px - 32px - 46px)",
          minHeight: 480,
          border: 0,
          borderRadius: 8,
          background: "#0f0f0f",
        }}
      />
    </div>
  );
}
