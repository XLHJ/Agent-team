package rpc

import (
	"encoding/json"
	"fmt"
)

// Standard JSON-RPC 2.0 codes.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

// Business codes (go into error.data.code).
const (
	BusProtocolVersionMismatch = "PROTOCOL_VERSION_MISMATCH"
	BusInstanceIDConflict      = "INSTANCE_ID_CONFLICT"
	BusCapabilityNotDeclared   = "CAPABILITY_NOT_DECLARED"
	BusTaskNotFound            = "TASK_NOT_FOUND"
	BusApprovalNotFound        = "APPROVAL_NOT_FOUND"
	BusApprovalAlreadyResolved = "APPROVAL_ALREADY_RESOLVED"
)

// ErrorData is the error.data payload.
type ErrorData struct {
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

// ErrorObject is a JSON-RPC error object.
type ErrorObject struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *ErrorData `json:"data,omitempty"`
}

// Response is a JSON-RPC response frame.
type Response struct {
	JSONRPC         string          `json:"jsonrpc"`
	ProtocolVersion string          `json:"protocol_version"`
	ID              json.RawMessage `json:"id"`
	Result          any             `json:"result,omitempty"`
	Error           *ErrorObject    `json:"error,omitempty"`
}

// NewError builds an error object.
func NewError(code int, msg string, data *ErrorData) *ErrorObject {
	return &ErrorObject{Code: code, Message: msg, Data: data}
}

// NewBusiness builds a business-coded error object.
func NewBusiness(busCode, msg string) *ErrorObject {
	return NewError(CodeInvalidRequest, msg, &ErrorData{Code: busCode})
}

// ErrReply builds a Response carrying err for id.
func ErrReply(id json.RawMessage, err *ErrorObject) *Response {
	return &Response{JSONRPC: "2.0", ProtocolVersion: "0.1", ID: id, Error: err}
}

// OKReply builds a success Response for id.
func OKReply(id json.RawMessage, result any) *Response {
	if result == nil {
		result = map[string]any{}
	}
	return &Response{JSONRPC: "2.0", ProtocolVersion: "0.1", ID: id, Result: result}
}

// String renders the error for logs.
func (e *ErrorObject) String() string {
	if e.Data != nil && e.Data.Code != "" {
		return fmt.Sprintf("%d %s (%s)", e.Code, e.Message, e.Data.Code)
	}
	return fmt.Sprintf("%d %s", e.Code, e.Message)
}
