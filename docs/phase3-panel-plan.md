# 第三阶段落地规划说明书：桌面面板

- 状态：规划级（落地准备），**不是图形化方案**，不等于编码授权
- 日期：2026-11-13
- 上游文档：`docs/phase1-protocol-v0.1.md`（D1 协议契约）、阶段报告一/二/三
- 门禁关系：本说明书解决"做什么、依赖什么、接口长什么样、按什么顺序做"；**布局/视觉/交互（阶段三报告第八章 8.1–8.5）仍须在编码前另行确认**。本文档标注的「待确认」项即第八章清单的输入。

---

## 一、现状核对（落地前提）

| 项 | 现状（2026-11-13 工作区核对） | 对第三阶段的影响 |
|---|---|---|
| 协议 schemas（D2） | `protocol/schemas/` 14 个 schema 已交付 | panel.* 转译层可直接引用 |
| TS 类型（D3） | `protocol/types/` 仅 tsconfig，`protocol.ts` 未生成 | 面板渲染进程需要对应 TS 类型，须补齐 |
| Hub 实现（D5/D6/D7） | `hub/cmd/hub/main.go` 为 12 行占位骨架，`internal/{ws,rpc,registry,tasks,approvals}` 全部为空目录 | **P0 前置**：第一、二阶段 Hub 功能未落地，panel.* 无可挂载的状态机与注册表 |
| Harness/Plugin SDK（第二阶段） | 未见代码 | P0 前置：Agent 看板/事件流的实时数据来源 |

结论：第三阶段启动的硬前提是 **第一阶段 T1–T12 在 Go Hub 上全部通过 + 第二阶段接管框架至少能注册一个实例**。否则面板只有空壳可显示。落地顺序见第七章。

---

## 二、交付物清单

| 编号 | 交付物 | 形式 | 验收方式 |
|---|---|---|---|
| D3-1 | panel.* 接口契约文档（本章第四节的定稿版） | Markdown | 人工评审，冻结后 UI 才开工 |
| D3-2 | Hub 面板通道：/panel WS + token/Origin 校验 + hub.port 运行时文件 | Go 代码 | 自测项 P3-01~03 |
| D3-3 | Hub panel.* 方法实现（订阅/推送/查询/命令转译） | Go 代码 | 自测项 P3-04~07 |
| D3-4 | Electron 壳：spawn Hub、token 注入、窗口与全局弹窗层 | TS 代码 | 手动验证启动链路 |
| D3-5 | 渲染进程基建：Vite + React 19 工程、RPC 客户端、订阅管理、断线横幅、全量刷新 | TS 代码 | 自测项 P3-08 |
| D3-6 | 四视图：Agent 看板 / 任务看板 / 审批中心 / 事件流 | TS 代码 | 逐视图验收点（第六章） |
| D3-7 | 协议 TS 类型（`protocol/types/protocol.ts`，D3 补齐） | TS | `tsc --noEmit` 零错误 |
| D3-8 | 第三阶段自测清单与执行记录 | Markdown | 全部通过 |

---

## 三、技术栈锁定表

原则：报告第二章的选型不推翻，只把「待定」变成「可执行决策 + 待确认项」。版本号统一在**开发机**上用 `npm view <pkg> version` 查最新稳定版后锁定写入 package.json（本机 npm 源不可达，无法代查）。

| 层 | 选型 | 锁定策略与说明 |
|---|---|---|
| 桌面壳 | Electron | 锁开发机当时最新 stable；仅主进程用 Node 能力，渲染进程 sandbox 开启 |
| 前端框架 | React 19 + TypeScript strict | 函数组件 + Hooks，禁类组件 |
| 构建 | Vite | dev 态 HMR（渲染进程加载 http://localhost:5173），生产态 build 产物由主进程 loadFile |
| UI 组件 | Ant Design | **主版本待确认**：优先 antd v6（官方已发布、面向 React 19）；若 v6 与依赖组件冲突则回退 v5 最新 |
| HITL 组件 | **待确认（见 10.1）** | 报告所写 `@ant-design/agentic-ui` 在线无法查证；候选替代：蚂蚁官方 `Ant Design X`（antdx）。审批中心只消费其基础交互组件，不深度绑定 |
| DAG 可视化 | `@xyflow/react` + `elkjs` | **第三阶段只锁版本不写代码**（第三阶段不渲染 DAG，这是第五阶段依赖）；避免 PhantomJS 类传递依赖污染构建 |
| 状态管理 | 推荐 Zustand + TanStack Query 组合 | Zustand 管 Hub 权威状态镜像与事件缓冲；TanStack Query 管 panel.* 拉取缓存。**待图形化方案确认** |
| 样式 | 待图形化方案确认（8.3） | 倾向 CSS Modules + antd token，避免引入 Tailwind 与 antd 的样式冲突 |
| WebSocket 客户端 | 渲染进程原生 `WebSocket` | JSON-RPC 帧封装自实现；不引 socket.io |

---

## 四、panel.* 接口契约（草案 → 定稿基线）

约定：面板与 Hub 复用 JSON-RPC 2.0 帧（`jsonrpc`、`id`、`method`、`params`/`result|error`），**不带 `protocol_version`**——版本治理属于插件协议，面板通道以本契约文档为版本依据。错误码复用插件协议业务错误码 + JSON-RPC 标准码。

### 4.1 传输与安全

| 项 | 规定 |
|---|---|
| 监听 | Hub 只监听 `127.0.0.1:<动态端口>` |
| 端口发现 | Hub 启动时把端口写入 `<用户主目录>/.agent-control-plane/hub.port`（`os.UserHomeDir()`，权限 0600；Windows/macOS 各自主目录语义由 Go 抽象处理） |
| token | Electron 主进程 spawn Hub 时生成随机 token 并注入环境变量 `ACP_PANEL_TOKEN`，同一 token 通过 preload 注入渲染进程；**不落盘** |
| 握手 | `ws://127.0.0.1:<port>/panel?token=<t>`；token 错误 → HTTP 401 拒绝升级 |
| Origin 校验 | 白名单：生产 `file://`（Chromium 发送字符串 `"null"`，按字面量接受）+ 开发态 `http://localhost:5173`。其余一律拒绝。技术疑点见 10.3 |
| 面板独占 | 同一 token 串行会话；第二连接到达时 Hub 拒绝（单面板实例约束，报告 1.3） |

### 4.2 面板 → Hub（请求/响应）

| 方法 | params（草案） | result（草案） | 说明 |
|---|---|---|---|
| `panel.subscribe` | `{ types?: string[] }`（通配 `task.*`，缺省全量） | `{}` | 同一连接可重复调用收窄过滤 |
| `panel.unsubscribe` | `{}` | `{}` | |
| `panel.agents.list` | `{}` | `{ instances: AgentInstance[] }` | 注册表快照 |
| `panel.tasks.list` | `{ status?, instance_id?, workflow_run_id?, since?, cursor?, limit?=50 }` | `{ tasks: TaskSummary[], next_cursor? }` | Hub 侧分页（cursor） |
| `panel.tasks.get` | `{ task_id }` | `TaskDetail` | 含 prompt/input_context/时间线/进度/artifacts/raw_output/report |
| `panel.tasks.create` | `{ instance_id, prompt, report_prompt?, timeout_ms?, approval_policy? }` | `{ task_id }` | Hub 校验实例 READY 且声明 task.dispatch（CAPABILITY_NOT_DECLARED），生成 task_id 后转译 task.dispatch |
| `panel.tasks.cancel` | `{ task_id, reason? }` | `{}` | 转译 task.cancel；任务不存在 → TASK_NOT_FOUND |
| `panel.approvals.list` | `{ status?: "pending"\|"resolved", instance_id?, task_id?, cursor?, limit? }` | `{ items: Approval[], next_cursor? }` | **第三阶段边界**：已决历史仅本次 Hub 运行内存态（审批 SQLite 持久化属第四阶段），面板需注明"重启后历史清空" |
| `panel.approvals.respond` | `{ approval_id, decision, modified_params?, reason?, exempt? }` | `{}` | 转译 approval.respond；`exempt=true` 为第四阶段豁免语义占位，第三阶段忽略 |
| `panel.intake.candidates` | `{}` | `{ candidates: IntakeCandidate[] }` | 第二阶段接入管理器待确认列表 |
| `panel.intake.confirm` | `{ name }` | `{}` | 触发接管 |
| `panel.intake.ignore` | `{ name }` | `{}` | 本次运行不再弹窗 |

`AgentInstance`：`{ instance_id, agent_type, agent_version?, capabilities[], source: "cli"|"plugin", conn_status: "READY"|"UNREACHABLE"|"DISCONNECTED", run_status: "idle"|"busy"|"waiting_approval"|"error", current_task_id?, last_heartbeat_at }`

### 4.3 Hub → 面板（推送）

| 方法 | params | 说明 |
|---|---|---|
| `panel.event` | `{ event_type, ts, payload }` | 唯一推送帧。`event_type` 取值：`task.dispatch/progress/completed/failed`、`approval.requested/resolved`、`agent.status`、`intake.candidate` |

转译映射（Hub 内部）：插件上行事件 → `panel.event` 一对一镜像；不新增聚合事件。`task.dispatch` 由 Hub 主动下发时（panel.tasks.create）同样镜像，保证面板看到自己触发的变更。

### 4.4 定稿流程

图形化方案确认时若对以上字段提出新需求（如"按 Agent 分组统计"），**先改本契约再进 UI 开发**。定稿后本节表头改为「契约 v3.0（冻结）」。

---

## 五、面板工程结构

```
agent-control-plane/
├── panel/
│   ├── electron/
│   │   ├── main.ts            # 窗口生命周期、spawn Go Hub、token 生成、端口文件读取
│   │   ├── hub.ts             # Hub 进程管理：启动/健康检查/退出联动/崩溃重启提示
│   │   └── preload.ts         # contextBridge 暴露 { hubPort, panelToken }，其余全关
│   ├── src/
│   │   ├── app/               # 路由、全局布局壳、断线横幅、弹窗层（intake/断线提醒）
│   │   ├── api/               # rpc.ts（帧封装）、methods.ts（panel.* 类型化封装）
│   │   ├── stores/            # connection / agents / tasks / approvals / events
│   │   ├── views/
│   │   │   ├── agents/
│   │   │   ├── tasks/
│   │   │   ├── approvals/
│   │   │   └── events/
│   │   ├── components/        # 与视图解耦的通用件（状态徽标、倒计时、虚拟列表包装）
│   │   └── types/             # 引用 protocol/types/protocol.ts + panel 契约类型
│   ├── package.json           # 版本锁定处
│   └── vite.config.ts
└── docs/
    └── phase3-panel-plan.md   # 本文档
```

约束：渲染进程不直接 `require` Node 模块（contextIsolation + sandbox 全开）；除 preload 暴露的两个连接参数外，`window.electron` 无其他面。

---

## 六、数据流与状态管理（报告第五章的落地规则）

1. **三分类落点**：
   - Hub 权威状态 → `stores/` 中的镜像 store，只允许被 `panel.event` 与全量快照两个入口写入。
   - 面板本地 UI 状态 → 组件 useState / URL 参数，不入全局 store。
   - 派生状态 → selector/memo（如按 Agent 分组任务），不落 store。
2. **禁止乐观更新**：所有命令按钮提交后进入"处理中"态，以对应 `panel.event` 或方法 result 到达为解除条件；错误则回显错误码。
3. **重连时序（固定顺序）**：WS `onclose` → 横幅 +「重新连接」按钮 → 用户点击 → 新连接成功 → 依次 `panel.agents.list`、`panel.tasks.list`、`panel.approvals.list` 全量重建 → 重新 `panel.subscribe` → 撤横幅。期间事件缓冲不清空但标记"断线间隙"。
4. **事件缓冲**：`events` store 上限 1000 条（环形覆盖），超出丢弃最旧并计数。具体数值待图形化方案确认（8.4）。
5. **倒计时规则**：审批倒计时由面板用 Hub 下发的 `timeout_at` 本地渲染，归零后**不发任何裁决**，仅调 `panel.approvals.list` 刷新该条；裁决权在 Hub。

---

## 七、任务分解与落地顺序

| # | 任务 | 依赖 | 产出 | 验收 |
|---|---|---|---|---|
| T3.0 | **前置**：第一阶段 Hub 功能落地（ws/rpc/registry/tasks/approvals 五模块），T1–T12 自测通过；TS 类型补齐 | 一、二阶段规划 | 可运行 Go Hub | T1–T12 |
| T3.1 | Hub 面板通道：/panel WS、token/Origin 校验、hub.port 文件、单面板独占 | T3.0 | D3-2 | P3-01~03 |
| T3.2 | Hub panel.* 方法：订阅推送 + 6 组查询/命令 + 协议转译 | T3.1 | D3-3 | P3-04~07 |
| T3.3 | Electron 壳工程：spawn/health/退出联动、preload、Vite dev/prod 双模式 | 无（可与 T3.1 并行） | D3-4 | 手动链路 |
| T3.4 | 渲染进程基建：RPC 客户端、订阅管理、三分类 store、断线横幅、全量刷新 | T3.2、T3.3 | D3-5 | P3-08 |
| T3.5 | Agent 看板 | T3.4 | D3-6 | 视图验收 A |
| T3.6 | 任务看板（列表/详情/创建/取消） | T3.4 | D3-6 | 视图验收 B |
| T3.7 | 审批中心（待决/已决/详情） | T3.4 | D3-6 | 视图验收 C |
| T3.8 | 事件流 | T3.4 | D3-6 | 视图验收 D |
| T3.9 | 全局弹窗层（intake 候选确认、实例断线提醒） | T3.4 | D3-6 | P3-09 |
| T3.10 | 自测清单执行与记录 | 全部 | D3-8 | P3 全项 |

说明：T3.3 不依赖 Hub，可先行搭壳。T3.5–T3.8 的**视觉与交互细节**在图形化方案确认后才进入对应视图编码；在此之前各任务只完成数据层 + 骨架路由。

---

## 八、逐视图验收点（规划级）

- **A Agent 看板**：快照 + 增量一致；`cli:`/`plugin:` 来源字段可区分；DISCONNECTED 出现"重试连接"且只触发一次（第二阶段语义）；无删除实例操作。
- **B 任务看板**：手动创建任务（选 READY 实例、填 prompt/可选 report_prompt/timeout）→ 详情可见 prompt、input_context、进度时间线、artifacts、raw_output/report；取消按钮仅运行中任务可见；无"重试失败任务"。
- **C 审批中心**：待决按剩余时间升序；裁决四操作（含豁免按钮占位，提示第四阶段生效）；冲突裁决回显 `APPROVAL_ALREADY_RESOLVED` 并刷新；已决历史标注"本次运行"。
- **D 事件流**：倒序、三类过滤、点击跳转对应详情；缓冲上限生效；不入库，重启即空。
- **全局**：断线横幅常驻直至手动重连成功；intake 弹窗可多选、可忽略（本次运行内不再弹）。

---

## 九、自测清单（P3 系列）

| # | 操作 | 预期 |
|---|---|---|
| P3-01 | 无 token 连接 /panel | 升级被拒（401） |
| P3-02 | 伪造 Origin 连接 | 被拒 |
| P3-03 | 正常连接后第二连接 | 被拒（单面板约束） |
| P3-04 | `panel.agents.list` | 与 GET /registry 快照一致 |
| P3-05 | 订阅 `task.*` 后经 `panel.tasks.create` 派发 | 面板收到 task.dispatch/progress/completed 镜像 |
| P3-06 | 对只声明 observe 的实例 create | 返回 CAPABILITY_NOT_DECLARED |
| P3-07 | 对已决审批再次 respond | 返回 APPROVAL_ALREADY_RESOLVED |
| P3-08 | 杀掉 Hub 进程再重启重连 | 横幅出现 → 手动重连 → 全量刷新后视图与 Hub 一致 |
| P3-09 | intake 有候选时 | 弹窗出现；confirm 后注册表出现该实例；ignore 后本次不再弹 |
| P3-10 | 事件流灌入 >1000 条 | 缓冲稳定在上限，内存不增长 |

---

## 十、技术疑点查证结论与待确认清单

### 10.1 已有结论（本次查证）

| 疑点 | 结论 |
|---|---|
| `@ant-design/agentic-ui` | **未能查证到**（搜索结果异常 + 本机 npm 源不可达）。不能确认其存在与维护状态。候选替代：蚂蚁官方 AI 组件库 **Ant Design X（antdx）**，其 HITL/对话类组件可覆盖审批中心需求。**待用户确认** |
| antd × React 19 | antd 官方已发布 v6 主打 React 19 支持；v5 最新版亦兼容。倾向 v6，**开发机核实后锁定** |
| ReactFlow | 库已迁移至 `@xyflow/react`（v12+），配套 elkjs 做自动布局。第三阶段仅锁版本，不写代码 |
| Electron × WebSocket Origin | 原生 WS **无法自定义 Origin/头**，token 必须走 URL query（已在 4.1 采用）；`file://` 页面发起的请求 Origin 为字符串 `"null"`，Hub 侧按字面量白名单；开发态加 `http://localhost:5173` |
| Electron 版本 | 锁开发机当时最新 stable（本机无法查证具体号） |
| npm 源可达性 | 本机 HTTP 通道到 registry.npmjs.org 的重定向目标（HTTPS）不可达 → 所有 npm 版本查询、`npm install`、`tsc --noEmit` 必须在开发机执行（与一阶段结论一致） |

### 10.2 待用户确认（进入图形化方案前的决策项）

1. HITL 组件基座：接受 antdx 替代 agentic-ui，或提供 agentic-ui 的确切出处？
2. antd 主版本：v6 还是 v5？
3. 状态管理组合（Zustand + TanStack Query）与样式方案（CSS Modules + token）：按本推荐执行还是另行确认？
4. 已决审批历史第三阶段仅内存态（重启清空），SQLite 化随第四阶段——是否接受？

### 10.3 风险与注意事项（报告第九章之外的补充）

- **P0 依赖风险**：Hub 仍是占位骨架，第三阶段一切面板功能都压在 T3.0 上；建议 T3.0 未完成前不开任何面板编码。
- **Origin="null" 的取舍**：接受 `file:// → "null"` 是 Electron 生态现实做法，但白名单过宽（任何 file:// 页面）。由于叠加了随机 token + loopback + 单连接独占，风险可接受；若要收紧，后续改用自定义 `app://` 协议（Electron `registerSchemesAsPrivileged`）使 Origin 非空，列为增强项不阻塞本阶段。
- **hub.port 文件竞争**：多实例 Hub 同时启动会互相覆盖端口文件；第三阶段按单实例假设，启动时检测文件新鲜度（写入时间 < 5s 且进程存活）则报错退出。
- **事件洪泛**：某 Agent 高频 task.progress 时面板事件缓冲可能瞬时打满；1000 条环形 + `panel.subscribe` 支持按类型收窄可缓解，GUI 方案确认节流参数。

---

## 十一、与图形化方案门禁的边界（重申）

本说明书冻结的是：接口契约基线、工程结构、数据流规则、任务顺序、验收清单。
不冻结的是：布局、视觉、交互流程、组件样式、分页与缓冲的具体数值。
按阶段三报告第八章门禁：8.1–8.5（含 10.2 的四项）书面确认前，**不开始视图编码**；T3.0–T3.4 的非视觉部分可先行。
