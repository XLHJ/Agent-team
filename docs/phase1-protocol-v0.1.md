# 协议 v0.1 —— 方法清单与语义说明（D1）

- 状态：冻结草案（待评审）
- 日期：2026-11-13
- 适用：第一阶段 Hub 骨架；第二阶段真实插件按此反向验证
- 配套文档：`docs/phase1-version-rules.md`（D4 版本与兼容规则）、`protocol/errors.md`（错误码表）

---

## 一、总体约定

1. 传输：WebSocket 长连接，每条消息占一个**文本帧**。
2. 编码：JSON-RPC 2.0。每帧必含 `jsonrpc: "2.0"`、`method`；请求-响应式消息必含 `id`。
3. 响应帧必含 `jsonrpc`、`id`，以及 `result` 或 `error` 二者之一。
4. `protocol_version` 为**帧顶层字段**（与 `jsonrpc`/`method` 同级），值为 `"0.1"`。
5. 幂等：所有涉及状态变更的下行方法必带 `task_id` 或 `approval_id`，插件侧负责去重。v0.1 **不引入独立的 `idempotency_key`**，`task_id` 即幂等键（与总规划 5.4 的差异见第七节）。
6. 心跳采用**请求-响应模式**（按第一阶段报告 4.4）：`plugin.heartbeat` 携带 `id`，Hub 返回空 `result`。报告 3.3 中"无响应"的表述以 4.4 为准。
7. 第一阶段 Hub 不主动派发任务；`task.dispatch` 仅在自测清单中手动构造下发。
8. 每个方法的 `params` 校验规则由 `protocol/schemas/<method>.schema.json` 定义（D2，任务 3 交付）。

---

## 二、protocol_version 字段的位置约定（本次冻结决策）

| 位置 | 必填性 | 说明 |
|---|---|---|
| 帧顶层（`jsonrpc` 同级） | 必填，每帧 | Hub 路由层校验的主依据 |
| `plugin.register` 的 `params` 内 | 必填（仅此方法） | 注册时显式声明，**必须与帧顶层一致**，不一致返回 `-32602` 并拒绝注册 |

理由：报告 3.1 要求"每条消息必须包含 protocol_version"→ 帧顶层；4.4 要求路由层校验该字段 → 路由层读顶层；3.3 的 register 字段表把 `protocol_version` 列为 params 必填 → params 保留副本。三处条款同时满足的唯一读法即"顶层权威 + register 副本"。冗余成本仅一个字符串字段。

---

## 三、上行方法（插件 → Hub）

### 3.1 plugin.register

- 语义：连接建立后的第一条消息；声明 Agent 类型、实例、能力，换取注册确认。
- params 必填：`agent_type`（string）、`instance_id`（string，全局唯一）、`protocol_version`（string，须等于帧顶层）、`capabilities`（array，取值见 3.10 能力枚举）。
- params 可选：`agent_version`（string）、`metadata`（object，自由字段，Hub 不解析其内容）。
- 响应：`plugin.registered`（下行）。
- 失败：主版本不匹配 → `PROTOCOL_VERSION_MISMATCH` 且关闭连接；`instance_id` 重复 → `INSTANCE_ID_CONFLICT`；字段缺失/类型错误 → `-32602`。
- 时序约束：注册前连接上到达的任何其他方法 → `-32600` 并关闭连接（T2）。

### 3.2 plugin.heartbeat

- 语义：保活。Hub 收到即更新该实例 `lastHeartbeatAt`。
- params 必填：`instance_id`、`timestamp`（ISO8601）。
- 响应：空 `result`（请求-响应模式）。
- 无心跳超时：连续 3 个周期（默认 90 秒）无任何消息 → 连接标记 `UNREACHABLE`，不立即断开。

### 3.3 agent.status

- 语义：Agent 自身运行状态变更上报（注册表展示与看板数据源）。
- params 必填：`instance_id`、`status`（枚举 `idle / busy / waiting_approval / error`）。
- params 可选：`current_task_id`。
- 响应：空 `result`。

### 3.4 task.progress

- 语义：任务执行中进度上报；首次到达使任务进入 `running`。
- params 必填：`task_id`、`progress`（number 0–100）、`step`（string）。
- params 可选：`token_used`（integer）、`eta_ms`（integer）。
- 响应：空 `result`。
- 状态机交互：仅 `dispatched`/`running` 状态的任务接受；对终态任务上报 → 记录警告 + 返回 `TASK_NOT_FOUND` 或忽略，不影响 Hub 稳定性。

### 3.5 task.completed

- 语义：任务成功终态上报。首次到达使任务进入 `completed`。
- params 必填：`task_id`、`raw_output`（string）。
- params 可选：`report`（string，仅当注册声明 `report.generate`；缺失时协作链场景按第五阶段规范处理，v0.1 只记录字段是否存在）、`artifacts`（array of string）。
- 响应：空 `result`。
- 时序约束：对不存在或已终态的 task_id 上报 → 警告 + `TASK_NOT_FOUND`/忽略。

### 3.6 task.failed

- 语义：任务失败终态上报。
- params 必填：`task_id`、`error_code`（string）、`error_message`（string）。
- params 可选：`partial_output`（string）。
- 响应：空 `result`。
- 状态机交互：`completed` 与 `failed` 互斥，先到先定终态，后到者警告+忽略。

### 3.7 approval.requested

- 通道：独立审批通道，不混入进度流（核心设计原则四）。
- params 必填：`approval_id`（string，**插件生成**，Hub 校验唯一性）、`task_id`、`instance_id`、`operation_type`（string）、`risk_level`（枚举 `low / medium / high`）、`context_summary`（string）。
- params 可选：`timeout_ms`（integer，缺省用 Hub 默认）。
- 响应：空 `result`（入队确认，裁决走下行 approval.respond）。
- Hub 行为：入队（status=pending），记录 instanceId 供路由；**第一阶段不做超时自动裁决**，只记录 createdAt。

### 3.8 log.event（可选方法）

- 语义：结构化日志，面板按需订阅；v0.1 Hub 只记录是否收到，不做存储。
- params 必填：`level`（枚举 debug/info/warn/error）、`message`（string）。
- params 可选：`task_id`、`instance_id`、`fields`（object）。
- 响应：空 `result`。

### 3.9 agent.status 与 task.progress 的状态字段差异

- `agent.status` 是**实例级**状态（实例在做什么的粗粒度自报）；`task.progress` 是**任务级**进度（某一任务的完成度）。
- 实例状态与任务状态是两个独立的状态机，Hub 不做联动推断（例如 busy 不隐含任何任务状态）。

### 3.10 能力枚举与语义

| 能力 | 语义 |
|---|---|
| observe | 允许 Hub 下发观察类查询 |
| task.dispatch | 允许 Hub 下发任务 |
| session.interrupt | 允许 Hub 下发中断会话 |
| approval | 允许参与审批通道（收 approval.respond） |
| report.generate | 声明后允许在 task.completed 中返回非空 report 字段 |

### 未声明能力的默认行为

- 未声明 `task.dispatch` 的实例：Hub 派发时拒绝下发，返回 `CAPABILITY_NOT_DECLARED`（T7）。
- 未声明 `approval` 的实例发 approval.requested → Hub 记录异常，正常入队但审计标记；未声明 `report.generate` 发含 report 的 completed → 同样审计标记。
- "Hub 不向不支持 session.interrupt 的插件下发中断指令"照旧成立。

---

## 四、下行方法（Hub → 插件）

### 4.1 plugin.registered

- 语义：注册确认。
- params 必填：`instance_id`、`session_token`（v0.1 为占位值）、`server_time`（ISO8601）。
- params 可选：`policy`（object，v0.1 恒为空对象）。
- 方向：Hub 主动下发，无响应。Hub 主动下发的帧**无 id 字段**（JSON-RPC Notification）。

### 4.2 task.dispatch

- 语义：下发任务。第一阶段仅自测清单手动构造，Hub 不主动派发。
- params 必填：`task_id`（**Hub 生成**）、`prompt`（string）、`timeout_ms`（integer）。
- params 可选：`report_prompt`（string）、`input_context`（string）、`approval_policy`（array of string）。
- 幂等：插件侧按 task_id 去重；重连后重复派发由第二阶段重连恢复逻辑处理，v0.1 无独立 idempotency_key。
- 能力守卫：目标实例未声明 task.dispatch 能力 → `CAPABILITY_NOT_DECLARED`，不产生任务记录。

### Hub 生成的 task_id / 插件生成的 approval_id

- `task_id` 由 Hub 生成（Hub 是任务的唯一创建者）。
- `task_id` Hub 生成 + `approval_id` 插件生成，原因：审批由插件侧操作触发，发起方生成 ID 可天然去重（插件重试重发同一 approval.requested 时 Hub 可按 approval_id 去重）。

### 4.3 task.cancel / session.interrupt / config.update

- `task.cancel`：必填 `task_id`，可选 `reason`。能力守卫不适用（它是对已有任务的指令）。
- `session.interrupt`：必填 `instance_id`，可选 `reason`。能力守卫：目标实例未声明该能力 → 不下发。
- `config.update`：必填 v0.1 无；params 为 `policy`（object，v0.1 内容不强约束）。
- 三者均为 Hub 主动下发的 Notification（无 id）。

### 4.4 approval.respond

- 裁决下行。params 必填：`approval_id`、`decision`（枚举 approve/deny/modify）。
- 可选：`modified_params`（object，仅 decision=modify 时必填）。
- 可选：`reason`（string）。
- 去重语义：迟到的重复裁决 → 返回 `APPROVAL_ALREADY_RESOLVED`（对面板入口）；
- Hub 收到 approval.respond 不回执（Notification）。插件收到后**不回执**，执行裁决即可。
- v0.1 路由：按 approval.requested 时的 instanceId 找原连接回传；原连接已断 → 保留终态记录，在 Hub 内存审批对象上标记 delivery_status = DELIVERY_FAILED（该字段属 Hub 内部结构，不进协议字段），并写入审计日志。

---

## 五、方向与响应总表

| 方法 | 方向 | 响应模式 | 去重/幂等键 |
|---|---|---|---|
| plugin.register | 上行 | JSON-RPC result/error + 另发 plugin.registered | instance_id 唯一性 |
| plugin.heartbeat | 上行 | 空 result | 无 |
| agent.status | 上行 | 空 result | 无 |
| task.progress | 上行 | 空 result | 无 |
| task.completed | 上行 | 空 result | task_id（终态幂等，先到先定） |
| task.failed | 上行 | 空 result | task_id（终态幂等，先到先定） |
| approval.requested | 上行 | 空 result（入队确认） | approval_id 唯一性 |
| log.event | 上行 | 空 result | 无 |
| plugin.registered | 下行 | Notification（无 id） | — |
| task.dispatch | 下行 | Notification（无 id） | task_id（插件侧去重） |
| task.cancel | 下行 | Notification（无 id） | task_id |
| session.interrupt | 下行 | Notification（无 id） | — |
| approval.respond | 下行 | Notification（无 id） | approval_id（先答者胜出） |
| config.update | 下行 | Notification（无 id） | — |

总计：8 上行 + 6 下行 = 14 个方法，第一阶段全部实现路由。

---

## 六、与总规划 5.4 的差异说明（idempotency_key）

- 总规划 5.4 要求"每个任务带 task_id + idempotency_key"。
- 第一阶段报告的字段清单没有 idempotency_key，且其"风险一"明确警告协议字段过度设计。
- 本次冻结决策：v0.1 不引入独立 idempotency_key，task_id 即幂等键。
- 理由：task_id 由 Hub 生成且全局唯一，已满足"重连后识别重复派发"的需求；独立 idempotency_key 的价值（区分同一逻辑任务的多次物理派发）在第一阶段无场景。
- 若第二阶段真实插件的重连恢复证明需要，按 D4 版本规则以次版本新增可选字段。

---

## 七、冻结范围声明

- 本文档冻结：方法名、方向、必填/可选字段、枚举值、错误码语义、响应模式。
- 不冻结：各字段 schema 细节（D2 交付）、TS 类型（D3 交付）、Hub 内部实现。
- 评审通过后本文档进入冻结状态；任何变更一律走 D4 版本与兼容规则。
