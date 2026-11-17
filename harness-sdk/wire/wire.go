// Package wire holds the JSON-RPC frame shapes and business error codes of
// protocol v0.1. The Hub and every SDK speak exactly this contract.
package wire

import (
	"encoding/json"
)

// ProtocolVersion is the frozen v0.1 protocol version.
const ProtocolVersion = "0.1"

// Upstream method names (plugin -> Hub).
const (
	MRegister          = "plugin.register"
	MHeartbeat         = "plugin.heartbeat"
	MAgentStatus       = "agent.status"
	MTaskProgress      = "task.progress"
	MTaskCompleted     = "task.completed"
	MTaskFailed        = "task.failed"
	MApprovalRequested = "approval.requested"
	MLogEvent          = "log.event"
	MUsageReport       = "usage.report"
)

// Downstream method names (Hub -> plugin).
const (
	MRegistered       = "plugin.registered"
	MTaskDispatch     = "task.dispatch"
	MTaskCancel       = "task.cancel"
	MSessionInterrupt = "session.interrupt"
	MApprovalRespond  = "approval.respond"
	MConfigUpdate     = "config.update"
)

// Business error codes (error.data.code).
const (
	BusProtocolVersionMismatch = "PROTOCOL_VERSION_MISMATCH"
	BusInstanceIDConflict      = "INSTANCE_ID_CONFLICT"
	BusCapabilityNotDeclared   = "CAPABILITY_NOT_DECLARED"
	BusTaskNotFound            = "TASK_NOT_FOUND"
	BusApprovalNotFound        = "APPROVAL_NOT_FOUND"
	BusApprovalAlreadyResolved = "APPROVAL_ALREADY_RESOLVED"
)

// Capabilities.
const (
	CapObserve          = "observe"
	CapTaskDispatch     = "task.dispatch"
	CapSessionInterrupt = "session.interrupt"
	CapApproval         = "approval"
	CapReportGenerate   = "report.generate"
)

// Approval modes (Phase 2 enum, behaviour filled by Phase 3 adapters).
const (
	ApprovalNativeForward = "native_forward"
	ApprovalToolHook      = "tool_hook"
)

// Frame is one JSON-RPC message in either direction.
type Frame struct {
	JSONRPC         string          `json:"jsonrpc"`
	ProtocolVersion string          `json:"protocol_version"`
	ID              json.RawMessage `json:"id,omitempty"`
	Method          string          `json:"method,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *ErrorObject    `json:"error,omitempty"`
}

// ErrorObject is a JSON-RPC error.
type ErrorObject struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *ErrorData `json:"data,omitempty"`
}

// ErrorData carries the business code and offending field.
type ErrorData struct {
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

// BusinessCode returns the business code, or "".
func (e *ErrorObject) BusinessCode() string {
	if e == nil || e.Data == nil {
		return ""
	}
	return e.Data.Code
}

// Request builds a request frame with a numeric id.
func Request(id uint64, method string, params any) ([]byte, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Frame{JSONRPC: "2.0", ProtocolVersion: ProtocolVersion, ID: json.RawMessage(itoa(id)), Method: method, Params: p})
}

// Notification builds an id-less frame (JSON-RPC notification).
func Notification(method string, params any) ([]byte, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Frame{JSONRPC: "2.0", ProtocolVersion: ProtocolVersion, Method: method, Params: p})
}

// Response builds a success response frame.
func Response(id json.RawMessage, result any) ([]byte, error) {
	r, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Frame{JSONRPC: "2.0", ProtocolVersion: ProtocolVersion, ID: id, Result: r})
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
