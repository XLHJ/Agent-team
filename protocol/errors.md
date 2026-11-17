# Protocol v0.1 error codes

- Status: frozen draft (under review)
- Date: 2026-11-13
- Companion: docs/phase1-protocol-v0.1.md (D1), protocol/version.md (D4)

## 1. Standard JSON-RPC codes

| Code | Meaning | When |
|---|---|---|
| -32700 | Parse error | Non-JSON text frame (T11) |
| -32600 | Invalid request | Non-register method before registration (T2); malformed envelope |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Schema validation failure (T12); path in error.data.field |
| -32603 | Internal error | Handler panic or unexpected failure |

## 2. Business codes (in error.data.code)

| Code | Meaning | When |
|---|---|---|
| PROTOCOL_VERSION_MISMATCH | Major version mismatch | register major != 0 (T4) |
| INSTANCE_ID_CONFLICT | instance_id already taken | Duplicate register (T5) |
| CAPABILITY_NOT_DECLARED | Capability missing | Dispatch/interrupt beyond declared capabilities (T7) |
| TASK_NOT_FOUND | Unknown task_id | Progress/completed/failed on unknown or terminal task (T8) |
| APPROVAL_NOT_FOUND | Unknown approval_id | Respond to unknown approval |
| APPROVAL_ALREADY_RESOLVED | Approval already decided | Second respond on terminal approval (T10) |

## 3. Response shape

    {"jsonrpc":"2.0","id":<id>,"error":{"code":<code>,"message":<text>,"data":{"code":"<BUSINESS_CODE>","field":"params.x"}}}

## 4. Rules

- Business code in error.data.code, never as top-level error.code.
- Validation failure must include error.data.field with the JSON path.
