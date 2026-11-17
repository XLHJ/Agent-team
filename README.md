# Agent++（仓库名 Agent-team）

一个面板，接入所有 Agent。**Agent++** 是插件化的多 Agent 控制面：Go 语言 Hub 负责协议、任务编排与持久化，Material 3 Expressive 桌面面板负责观测与操作，第三方 Agent 通过插件 SDK 即插即用。

## 功能总览

- **Agent 看板 / 任务看板 / 审批中心 / 事件流**：多 Agent 实例注册、任务派发与状态机、人工审批闸门、全量事件订阅
- **协作链（Workflows）**：多阶段任务链编排，支持链的导入/导出（chain.v1）
- **用量统计看板**：插件在任务结束时经 `usage.report` 上报各模型的 Token 消耗与性能指标（TTFT、生成速度、耗时），Hub 按小时/天聚合，面板支持 24h/7d/31d 窗口、按模型/来源分组与 CSV 导出（不含计费，价格由用户自理）
- **接入管理**：插件注册审批、能力声明校验（JSON Schema）
- **Dify 桥接**：Dify 工作流可通过 HTTP 节点/自定义工具把任务派给面板接入的 Agent（同步等待 / 异步轮询两种模式）
- **Kitesurf 桥接**：接入 Cloudflare 开源浏览器引擎，所有 Agent 可调用网页渲染（正文 + 截图）
- **内置子应用**：无限画布、AI 3D 建模（ApexForge）、文档编辑器（MCP Docs）、源码预览

## 目录结构

```
├── hub/            Go 控制面：WS JSON-RPC 协议、SQLite 持久化、任务状态机、
│                   审批引擎、用量聚合、Dify/Kitesurf HTTP 桥接、自检程序
├── panel/          桌面面板：React 19 + Vite + Electron，Material 3 Expressive
│   ├── src/        面板源码（统计看板、协作链、子应用视图等）
│   ├── canvas-app/     无限画布（vendored，MIT）
│   ├── apexforge-app/  AI 3D 建模（vendored，GPL-3.0，iframe 隔离）
│   ├── mcp-docs-app/   文档编辑器（vendored，MIT）
│   └── electron/       Electron 主进程（自动拉起 Hub，加载 dist/panel）
├── plugin-sdk/     插件 SDK（Go）：embedded / sidecar 两种接入形态
├── harness-sdk/    Agent 执行框架：harness-runner、fakeagent、Agent 检测签名
├── protocol/       协议 JSON Schema + TypeScript 类型
├── docs/           Dify 桥接 OpenAPI 工具 Schema 等
└── dist/           全部编译产物（单文件夹）
    ├── linux-amd64/     windows-amd64/
    ├── macos-amd64/     macos-arm64/
    │    └── hub / diag / harness-runner / fakeagent / sidecar-sample / embedded-sample
    └── panel/           面板网页构建（含三个子应用）
```

## 快速开始

**1. 启动 Hub**（以 Linux 为例，其他平台用 `dist/<平台>/hub`）：

```bash
dist/linux-amd64/hub \
  -addr 127.0.0.1:8080 \
  -token <自定义访问令牌> \
  -state-dir ./state \
  -schemas protocol/schemas \
  -signatures harness-sdk/detect/signatures
```

**2. 打开面板**：桌面端用 Electron（`panel/electron`，自动拉起 Hub 并加载 `dist/panel`）；或把 `dist/panel/` 交给任意静态服务器。

**3. 接入 Agent**：用 `plugin-sdk`（Go，embedded 或 sidecar）实现插件，按 `protocol/schemas` 声明能力并注册；示例见 `harness-sdk/cmd/fakeagent`。

**4. 用量上报**：插件在任务完成/失败时调用 `usage.report`（字段：模型、来源、输入/输出 Token、TTFT、Tokens/s、耗时），面板"统计看板"实时可见。

**5. Dify 工作流派任务给 Agent**：在 Dify「自定义工具」导入 `docs/dify-bridge.openapi.json`，鉴权头 `X-Hub-Token`；工作流即可调用 `dispatchTask`（`wait:true` 同步 / `wait:false` + `getTask` 轮询异步）。

**6. 网页渲染能力**：自行运行 [Kitesurf](https://github.com/cloudflare/kitesurf)（`pnpm wrangler dev`），在面板"无头浏览器"页配置其 CDP 端点；任意 Agent 调 `POST /kitesurf/v1/render` 获取页面正文与截图。

## 从源码构建

```bash
# Hub 与 SDK（Go ≥ 1.22）
cd hub && go build ./... && cd ..

# 面板（Node ≥ 20）：构建产物直接输出到 dist/panel/
cd panel && npm install --legacy-peer-deps && npm run build

# 跨平台二进制
cd hub && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" -o ../dist/windows-amd64/hub.exe ./cmd/hub
```

自检：`hub/cmd/selftest` ~ `selftest4`（协议、任务、审批、工作流链、用量、桥接全覆盖）。

## 版权与许可

- **无限画布**：MIT © 2026 basketikun —— 保留原作者信息与页面标识
- **ApexForge**：GPL-3.0 —— 以独立子应用 + iframe 隔离集成，源码与许可完整随附
- **MCP Docs**：MIT © 2026 Scott Persinger
- **Dify**：修改版 Apache-2.0 —— 本仓库不分发其源码，仅提供连接你自托管实例的入口（保留其 LOGO/版权要求）
- **Kitesurf**：Apache-2.0 © 2026 Cloudflare, Inc. —— 不 vendor、不修改，仅调用其公开 CDP/HTTP 接口
- 各第三方组件的 LICENSE/NOTICE 保留在其源码目录中；Agent++ 自有代码未声明许可证（默认保留所有权利）
