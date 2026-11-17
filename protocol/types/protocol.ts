/**
 * Protocol v0.1 types (D3).
 * One-to-one with protocol/schemas/*.schema.json.
 * Frame top-level carries protocol_version; params schemas validate params.
 */

export const PROTOCOL_VERSION = "0.1" as const;
export const JSONRPC_VERSION = "2.0" as const;

export type JsonRpcId = number | string;

export type ProtocolVersion = typeof PROTOCOL_VERSION;

export const ErrorCode = {
  PROTOCOL_VERSION_MISMATCH: "PROTOCOL_VERSION_MISMATCH",
  INSTANCE_ID_CONFLICT: "INSTANCE_ID_CONFLICT",
  CAPABILITY_NOT_DECLARED: "CAPABILITY_NOT_DECLARED",
  TASK_NOT_FOUND: "TASK_NOT_FOUND",
  APPROVAL_NOT_FOUND: "APPROVAL_NOT_FOUND",
  APPROVAL_ALREADY_RESOLVED: "APPROVAL_ALREADY_RESOLVED",
} as const;
export type ErrorCode = (typeof ErrorCode)[keyof typeof ErrorCode];

export interface JsonRpcRequest<TParams> {
  jsonrpc: typeof JSONRPC_VERSION;
  protocol_version: ProtocolVersion;
  method: string;
  id: JsonRpcId;
  params: TParams;
}

export interface JsonRpcNotification<TParams> {
  jsonrpc: typeof JSONRPC_VERSION;
  protocol_version: ProtocolVersion;
  method: string;
  params: TParams;
}

export interface JsonRpcErrorObject {
  code: number;
  message: string;
  data?: { code?: ErrorCode; field?: string; [k: string]: unknown };
}

export type JsonRpcResult<TResult> = {
  jsonrpc: typeof JSONRPC_VERSION;
  id: JsonRpcId | null;
  result: TResult;
};

export type JsonRpcErrorResponse = {
  jsonrpc: typeof JSONRPC_VERSION;
  id: JsonRpcId | null;
  error: JsonRpcErrorObject;
};

export type JsonRpcResponse<TResult> = JsonRpcResult<TResult> | JsonRpcErrorResponse;

export type Capability =
  | "observe"
  | "task.dispatch"
  | "session.interrupt"
  | "approval"
  | "report.generate";

export type AgentStatus = "idle" | "busy" | "waiting_approval" | "error";
export type RiskLevel = "low" | "medium" | "high";
export type ApprovalDecision = "approve" | "deny" | "modify";
export type LogLevel = "debug" | "info" | "warn" | "error";

// ---- Upstream params ----

export interface PluginRegisterParams {
  agent_type: string;
  instance_id: string;
  protocol_version: ProtocolVersion;
  capabilities: Capability[];
  agent_version?: string;
  metadata?: Record<string, unknown>;
}

export interface PluginHeartbeatParams {
  instance_id: string;
  timestamp: string;
}

export interface AgentStatusParams {
  instance_id: string;
  status: AgentStatus;
  current_task_id?: string;
}

export interface TaskProgressParams {
  task_id: string;
  progress: number;
  step: string;
  token_used?: number;
  eta_ms?: number;
}

export interface TaskCompletedParams {
  task_id: string;
  raw_output: string;
  report?: string;
  artifacts?: string[];
}

export interface TaskFailedParams {
  task_id: string;
  error_code: string;
  error_message: string;
  partial_output?: string;
}

export interface ApprovalRequestedParams {
  approval_id: string;
  task_id: string;
  instance_id: string;
  operation_type: string;
  risk_level: RiskLevel;
  context_summary: string;
  timeout_ms?: number;
}

export interface LogEventParams {
  level: LogLevel;
  message: string;
  task_id?: string;
  instance_id?: string;
  fields?: Record<string, unknown>;
}

export interface UsageCallDetail {
  model: string;
  source: string;
  input_tokens: number;
  output_tokens: number;
  ttft_ms: number;
  tokens_per_s: number;
  duration_ms: number;
}

export interface UsageReportParams {
  task_id: string;
  instance_id?: string;
  calls: UsageCallDetail[];
}

// ---- Downstream params ----

export interface PluginRegisteredParams {
  instance_id: string;
  session_token: string;
  server_time: string;
  policy?: Record<string, unknown>;
}

export interface TaskDispatchParams {
  task_id: string;
  prompt: string;
  timeout_ms: number;
  report_prompt?: string;
  input_context?: string;
  approval_policy?: string[];
}

export interface TaskCancelParams {
  task_id: string;
  reason?: string;
}

export interface SessionInterruptParams {
  instance_id: string;
  reason?: string;
}

export interface ApprovalRespondParams {
  approval_id: string;
  decision: ApprovalDecision;
  modified_params?: Record<string, unknown>;
  reason?: string;
}

export interface ConfigUpdateParams {
  policy: Record<string, unknown>;
}

// ---- Results (empty objects per D1) ----

export type EmptyResult = Record<string, never>;

export interface PluginRegisterResult {
  instance_id: string;
}

// ---- Upstream discriminated union (plugin -> Hub, request-response) ----

export type UpstreamRequest =
  | JsonRpcRequest<PluginRegisterParams> & { method: "plugin.register" }
  | JsonRpcRequest<PluginHeartbeatParams> & { method: "plugin.heartbeat" }
  | JsonRpcRequest<AgentStatusParams> & { method: "agent.status" }
  | JsonRpcRequest<TaskProgressParams> & { method: "task.progress" }
  | JsonRpcRequest<TaskCompletedParams> & { method: "task.completed" }
  | JsonRpcRequest<TaskFailedParams> & { method: "task.failed" }
  | JsonRpcRequest<ApprovalRequestedParams> & { method: "approval.requested" }
  | JsonRpcRequest<LogEventParams> & { method: "log.event" }
  | JsonRpcRequest<UsageReportParams> & { method: "usage.report" };

// ---- Downstream notifications (Hub -> plugin, no id) ----

export type DownstreamNotification =
  | JsonRpcNotification<PluginRegisteredParams> & { method: "plugin.registered" }
  | JsonRpcNotification<TaskDispatchParams> & { method: "task.dispatch" }
  | JsonRpcNotification<TaskCancelParams> & { method: "task.cancel" }
  | JsonRpcNotification<SessionInterruptParams> & { method: "session.interrupt" }
  | JsonRpcNotification<ApprovalRespondParams> & { method: "approval.respond" }
  | JsonRpcNotification<ConfigUpdateParams> & { method: "config.update" };

export type AnyProtocolMessage = UpstreamRequest | DownstreamNotification;
