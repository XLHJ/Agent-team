package rpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"agent-control-plane/hub/internal/approvalsv2"
	"agent-control-plane/hub/internal/panel"
	"agent-control-plane/hub/internal/registry"
	"agent-control-plane/hub/internal/store"
	"agent-control-plane/hub/internal/tasks"
	"agent-control-plane/hub/internal/workflows"
	"agent-control-plane/hub/internal/ws"
)

// Deps are router dependencies.
type Deps struct {
	Reg   *registry.Registry
	Tasks *tasks.Store
	Appr  *approvalsv2.Engine
	Wf    *workflows.Engine
	Pool  *ws.Pool
	Val   *Validator
	Log   *log.Logger
	Store *store.DB
	Bus   *panel.Bus
}

// Router dispatches upstream frames.
type Router struct {
	d Deps
}

// NewRouter builds a Router.
func NewRouter(d Deps) *Router {
	return &Router{d: d}
}

type envelope struct {
	JSONRPC         string          `json:"jsonrpc"`
	Method          string          `json:"method"`
	ProtocolVersion string          `json:"protocol_version"`
	ID              json.RawMessage `json:"id"`
	Params          json.RawMessage `json:"params"`
}

// Conn state tracked by the read loop.
type ConnState struct {
	Registered    bool
	PendingNotice any
}

var upstream = map[string]bool{
	"plugin.register": true, "plugin.heartbeat": true, "agent.status": true,
	"task.progress": true, "task.completed": true, "task.failed": true,
	"approval.requested": true, "log.event": true, "usage.report": true,
}

// Handle processes one raw frame; returns reply bytes (nil for notifications) and close flag.
func (r *Router) Handle(ctx context.Context, c *ws.Conn, st *ConnState, raw []byte) ([]byte, bool) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return marshal(ErrReply(nil, NewError(CodeParse, "parse error", nil))), false
	}
	if env.JSONRPC != "2.0" || env.ProtocolVersion == "" {
		return marshal(ErrReply(env.ID, NewError(CodeInvalidRequest, "invalid request envelope", nil))), !st.Registered
	}
	if !upstream[env.Method] {
		return marshal(ErrReply(env.ID, NewError(CodeMethodNotFound, "method not found", nil))), false
	}
	if !st.Registered && env.Method != "plugin.register" {
		return marshal(ErrReply(env.ID, NewError(CodeInvalidRequest, "first frame must be plugin.register", nil))), true
	}
	var frame any
	if err := json.Unmarshal(raw, &frame); err != nil {
		return marshal(ErrReply(env.ID, NewError(CodeParse, "parse error", nil))), false
	}
	if field, err := r.d.Val.Validate(env.Method, frame); err != nil {
		if field == "" && err.Error() == "method not found" {
			return marshal(ErrReply(env.ID, NewError(CodeMethodNotFound, "method not found", nil))), false
		}
		if field == "" {
			field = "params"
		}
		return marshal(ErrReply(env.ID, NewError(CodeInvalidParams, "invalid params", &ErrorData{Field: field}))), false
	}
	reply, closeAfter := r.dispatch(ctx, c, st, env)
	if len(env.ID) == 0 || string(env.ID) == "null" {
		return nil, closeAfter
	}
	if reply != nil {
		return marshal(reply), closeAfter
	}
	return marshal(OKReply(env.ID, nil)), closeAfter
}

func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (r *Router) onRegister(ctx context.Context, c *ws.Conn, st *ConnState, env envelope) (*Response, bool) {
	var p struct {
		AgentType       string         `json:"agent_type"`
		InstanceID      string         `json:"instance_id"`
		ProtocolVersion string         `json:"protocol_version"`
		Capabilities    []string       `json:"capabilities"`
		AgentVersion    string         `json:"agent_version"`
		Metadata        map[string]any `json:"metadata"`
	}
	_ = json.Unmarshal(env.Params, &p)
	if p.ProtocolVersion != env.ProtocolVersion {
		return ErrReply(env.ID, NewError(CodeInvalidParams, "protocol_version mismatch between envelope and params", &ErrorData{Field: "params.protocol_version"})), true
	}
	if major := strings.SplitN(p.ProtocolVersion, ".", 2)[0]; major != "0" {
		return ErrReply(env.ID, NewBusiness(BusProtocolVersionMismatch, "unsupported major protocol version")), true
	}
	mode := "native_forward"
	if m, ok := p.Metadata["approval_mode"].(string); ok && m == "tool_hook" {
		mode = m
	}
	rec := registry.Instance{
		AgentType:       p.AgentType,
		InstanceID:      p.InstanceID,
		ProtocolVersion: p.ProtocolVersion,
		Capabilities:    p.Capabilities,
		ApprovalMode:    mode,
		Status:          "idle",
	}
	// Same instance_id while a live connection exists is a conflict; a dead
	// record is rebound (Phase-2 7.1 reconnect with a stable instance id).
	if _, live := r.d.Pool.Get(p.InstanceID); live {
		return ErrReply(env.ID, NewBusiness(BusInstanceIDConflict, "instance_id conflict")), true
	}
	if r.d.Reg.Has(p.InstanceID) {
		r.d.Reg.Replace(rec)
	} else if err := r.d.Reg.Register(rec); err != nil {
		return ErrReply(env.ID, NewBusiness(BusInstanceIDConflict, "instance_id conflict")), true
	}
	c.InstanceID = p.InstanceID
	st.Registered = true
	r.d.Pool.Add(p.InstanceID, c)
	notif := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1", "method": "plugin.registered",
		"params": map[string]any{
			"instance_id":   p.InstanceID,
			"session_token": "placeholder",
			"server_time":   nowISO(),
			"policy":        map[string]any{},
		},
	}
	st.PendingNotice = notif
	_ = ctx
	r.d.Log.Printf("registered instance=%s type=%s caps=%v", p.InstanceID, p.AgentType, p.Capabilities)
	return OKReply(env.ID, nil), false
}

func (r *Router) onApprovalRequested(ctx context.Context, c *ws.Conn, env envelope) *Response {
	var p struct {
		ApprovalID     string           `json:"approval_id"`
		TaskID         string           `json:"task_id"`
		InstanceID     string           `json:"instance_id"`
		OperationType  string           `json:"operation_type"`
		RiskLevel      string           `json:"risk_level"`
		ContextSummary string           `json:"context_summary"`
		TimeoutMS      int              `json:"timeout_ms"`
		ParamsPreview  *json.RawMessage `json:"params_preview"`
	}
	_ = json.Unmarshal(env.Params, &p)
	if !r.d.Reg.HasCapability(p.InstanceID, "approval") {
		r.d.Log.Printf("warn: %s sent approval.requested without 'approval' capability", p.InstanceID)
	}
	row := store.ApprovalRow{
		ApprovalID: p.ApprovalID, TaskID: p.TaskID, InstanceID: p.InstanceID,
		OperationType: p.OperationType, RiskLevel: p.RiskLevel,
		ContextSummary: p.ContextSummary, TimeoutMS: int64(p.TimeoutMS),
		Mode: r.approvalMode(p.InstanceID),
	}
	if p.ParamsPreview != nil && len(*p.ParamsPreview) > 0 {
		row.ParamsPreview = sqlNull(string(*p.ParamsPreview))
	}
	out, exempted, err := r.d.Appr.Request(row, true)
	if err != nil {
		if err == store.ErrDupApproval {
			return OKReply(env.ID, nil) // duplicate id is idempotent
		}
		return ErrReply(env.ID, NewError(-32603, "approval persistence failed", nil))
	}
	if exempted && out != nil {
		note := map[string]any{
			"jsonrpc": "2.0", "protocol_version": "0.1",
			"method": "approval.respond",
			"params": map[string]any{
				"approval_id": p.ApprovalID, "decision": "approve",
				"reason": "exempted",
			},
		}
		_ = c.Send(ctx, note)
		return nil
	}
	_ = r.d.Tasks.WaitingApproval(p.TaskID)
	return nil
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
}

func (r *Router) dispatch(ctx context.Context, c *ws.Conn, st *ConnState, env envelope) (*Response, bool) {
	switch env.Method {
	case "plugin.register":
		return r.onRegister(ctx, c, st, env)
	case "plugin.heartbeat":
		var p struct {
			InstanceID string `json:"instance_id"`
		}
		_ = json.Unmarshal(env.Params, &p)
		r.d.Reg.Heartbeat(p.InstanceID)
		return nil, false
	case "agent.status":
		var p struct {
			InstanceID    string `json:"instance_id"`
			Status        string `json:"status"`
			CurrentTaskID string `json:"current_task_id"`
		}
		_ = json.Unmarshal(env.Params, &p)
		r.d.Reg.SetStatus(p.InstanceID, p.Status, p.CurrentTaskID)
		return nil, false
	case "task.progress":
		var p struct {
			TaskID string `json:"task_id"`
		}
		_ = json.Unmarshal(env.Params, &p)
		if err := r.d.Tasks.Progress(p.TaskID); err != nil {
			return ErrReply(env.ID, taskErr(err)), false
		}
		return nil, false
	case "task.completed":
		var p struct {
			TaskID    string   `json:"task_id"`
			RawOutput string   `json:"raw_output"`
			Report    string   `json:"report"`
			Artifacts []string `json:"artifacts"`
		}
		_ = json.Unmarshal(env.Params, &p)
		if err := r.d.Tasks.Complete(p.TaskID, p.RawOutput, p.Report, p.Artifacts); err != nil {
			return ErrReply(env.ID, taskErr(err)), false
		}
		if r.d.Wf != nil {
			r.d.Wf.OnTaskCompleted(ctx, p.TaskID, p.Report, p.RawOutput)
		}
		return nil, false
	case "task.failed":
		var p struct {
			TaskID       string `json:"task_id"`
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
			Partial      string `json:"partial_output"`
		}
		_ = json.Unmarshal(env.Params, &p)
		if err := r.d.Tasks.Fail(p.TaskID, p.ErrorCode, p.ErrorMessage, p.Partial); err != nil {
			return ErrReply(env.ID, taskErr(err)), false
		}
		if r.d.Wf != nil {
			r.d.Wf.OnTaskFailed(p.TaskID, p.ErrorCode, p.ErrorMessage)
		}
		return nil, false
	case "approval.requested":
		return r.onApprovalRequested(ctx, c, env), false
	case "log.event":
		return nil, false
	case "usage.report":
		return r.onUsageReport(env), false
	}
	return ErrReply(env.ID, NewError(CodeMethodNotFound, "method not found", nil)), false
}

// onUsageReport persists a task-level usage summary and notifies panels.
func (r *Router) onUsageReport(env envelope) *Response {
	var p struct {
		TaskID     string `json:"task_id"`
		InstanceID string `json:"instance_id"`
		Calls      []struct {
			Model        string  `json:"model"`
			Source       string  `json:"source"`
			InputTokens  int64   `json:"input_tokens"`
			OutputTokens int64   `json:"output_tokens"`
			TTFTMs       int64   `json:"ttft_ms"`
			TokensPerS   float64 `json:"tokens_per_s"`
			DurationMs   int64   `json:"duration_ms"`
		} `json:"calls"`
	}
	if err := json.Unmarshal(env.Params, &p); err != nil ||
		p.TaskID == "" || len(p.Calls) == 0 {
		return ErrReply(env.ID, NewError(CodeInvalidParams, "task_id and non-empty calls are required", nil))
	}
	if r.d.Store != nil {
		rep := store.UsageReportIn{TaskID: p.TaskID, InstanceID: p.InstanceID}
		for _, c := range p.Calls {
			rep.Calls = append(rep.Calls, store.UsageCall{
				Model: c.Model, Source: c.Source,
				InputTokens: c.InputTokens, OutputTokens: c.OutputTokens,
				TTFTMs: c.TTFTMs, TokensPerS: c.TokensPerS, DurationMs: c.DurationMs,
			})
		}
		if err := r.d.Store.UsageInsert(rep); err != nil {
			return ErrReply(env.ID, NewError(CodeInternal, "usage insert failed", nil))
		}
	}
	if r.d.Bus != nil {
		r.d.Bus.Publish(panel.SerEvent("usage.reported", map[string]any{
			"task_id": p.TaskID, "calls": len(p.Calls),
		}))
	}
	return nil
}

func taskErr(err error) *ErrorObject {
	if err == tasks.ErrIllegal {
		return NewError(CodeInvalidRequest, "illegal transition", &ErrorData{Code: BusTaskNotFound})
	}
	return NewError(CodeInvalidRequest, "task not found", &ErrorData{Code: BusTaskNotFound})
}

func (r *Router) approvalMode(instanceID string) string {
	if in, ok := r.d.Reg.Get(instanceID); ok && in.ApprovalMode != "" {
		return in.ApprovalMode
	}
	return "native_forward"
}

func sqlNull(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
