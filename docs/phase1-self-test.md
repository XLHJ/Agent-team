# Phase-1 protocol self-test checklist (D7)

- Status: frozen draft (under review)
- Date: 2026-11-13
- Two execution modes: automated (Go runner) and manual (wscat/curl, dev machine)

## 0. Automated runner (preferred)

    cd hub
    go run ./cmd/selftest ../protocol/schemas

Expected: every line `PASS ...`, final line `TOTAL fails=0`, exit code 0.
The runner boots an in-process hub on a random port; no external Hub needed.

## 1. Manual mode prerequisites

    cd hub && go run ./cmd/hub -addr 127.0.0.1:8080 -schemas ../protocol/schemas

`PORT=8080` below. `WS=ws://127.0.0.1:8080/ws`. Every frame carries top-level
`"jsonrpc":"2.0","protocol_version":"0.1"`.

## 2. Checklist

| # | Item | Action | Expected |
|---|---|---|---|
| T1 | Health | `curl -s localhost:8080/health` | 200 `{"status":"ok"}` |
| T2 | Pre-register reject | wscat 连上后直接发 plugin.heartbeat | `-32600`，连接被关闭 |
| T3 | Register | 发合法 plugin.register（caps 含 observe、task.dispatch） | 先收 result，再收 plugin.registered；`GET /registry` 可见 |
| T4 | Version mismatch | register 带 `protocol_version:"1.0"` | `PROTOCOL_VERSION_MISMATCH`，连接关闭 |
| T5 | instance_id 冲突 | 同一 instance_id 二次注册（另一连接） | `INSTANCE_ID_CONFLICT` |
| T6 | Heartbeat | 注册后发 plugin.heartbeat | 空 result；`GET /registry` 的 last_heartbeat_at 更新 |
| T7 | 能力越界 | 对仅 observe 的实例 `POST /selftest/tasks/dispatch` | 403 `CAPABILITY_NOT_DECLARED` |
| T8 | 任务状态流转 | dispatch 后发 task.progress，再发 task.completed | `GET /tasks/:id` 由 running 变 completed |
| T9 | 审批入队与路由 | 发 approval.requested，再 `POST /selftest/approvals/respond` | 队列 pending→resolved，原连接收到 approval.respond |
| T10 | 冲突裁决 | 对同一 approval_id 再次 respond | 409 `APPROVAL_ALREADY_RESOLVED` |
| T11 | 非法 JSON | 发非 JSON 文本帧 | `-32700` |
| T12 | Schema 校验 | register 缺必填字段 | `-32602`，error.data.field 指向字段 |

## 3. Sample frames (manual)

Register:

    {"jsonrpc":"2.0","protocol_version":"0.1","method":"plugin.register","id":1,
     "params":{"agent_type":"wscat","instance_id":"manual-1",
               "protocol_version":"0.1","capabilities":["observe","task.dispatch"]}}

Dispatch:

    curl -s -X POST localhost:8080/selftest/tasks/dispatch \
      -H 'Content-Type: application/json' \
      -d '{"instance_id":"manual-1","task_id":"t-1","prompt":"x","timeout_ms":5000}'

Progress / complete:

    {"jsonrpc":"2.0","protocol_version":"0.1","method":"task.progress","id":2,
     "params":{"task_id":"t-1","progress":50,"step":"half"}}

    {"jsonrpc":"2.0","protocol_version":"0.1","method":"task.completed","id":3,
     "params":{"task_id":"t-1","raw_output":"done"}}

Approval request / respond:

    {"jsonrpc":"2.0","protocol_version":"0.1","method":"approval.requested","id":4,
     "params":{"approval_id":"ap-1","task_id":"t-1","instance_id":"manual-1",
               "operation_type":"write_file","risk_level":"medium","context_summary":"x"}}

    curl -s -X POST localhost:8080/selftest/approvals/respond \
      -H 'Content-Type: application/json' \
      -d '{"approval_id":"ap-1","decision":"approve"}'

## 4. Result record

| # | Executed | Result | Evidence |
|---|---|---|---|
| T1-T12 | automated | 20 PASS / 0 FAIL (12 items, 20 assertions) | `go run ./cmd/selftest` output |
