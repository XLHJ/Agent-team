package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (h *Handler) taskCreate(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		InstanceID string `json:"instance_id"`
		Prompt     string `json:"prompt"`
		Report     string `json:"report_prompt"`
		InputCtx   string `json:"input_context"`
		TimeoutMS  int    `json:"timeout_ms"`
	}
	_ = json.Unmarshal(params, &p)
	if !h.d.Reg.Has(p.InstanceID) {
		return nil, bizErr("INSTANCE_NOT_FOUND", "no such instance")
	}
	if !h.d.Reg.HasCapability(p.InstanceID, "task.dispatch") {
		return nil, bizErr("CAPABILITY_NOT_DECLARED", "instance lacks task.dispatch")
	}
	taskID := fmt.Sprintf("pnl-%d", time.Now().UnixNano())
	h.d.Tasks.Dispatch(taskID, p.InstanceID, p.Prompt, p.TimeoutMS)
	dp := map[string]any{
		"task_id": taskID, "prompt": p.Prompt, "timeout_ms": p.TimeoutMS,
	}
	if p.Report != "" {
		dp["report_prompt"] = p.Report
	}
	if p.InputCtx != "" {
		dp["input_context"] = p.InputCtx
	}
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1",
		"method": "task.dispatch", "params": dp,
	}
	if !h.d.Pool.SendTo(ctx, p.InstanceID, note) {
		_ = h.d.Tasks.Fail(taskID, "DISPATCH_FAILED", "connection unavailable", "")
		return nil, bizErr("DISPATCH_FAILED", "instance unreachable")
	}
	h.d.Bus.Publish(SerEvent("task.dispatch", map[string]any{
		"task_id": taskID, "instance_id": p.InstanceID,
	}))
	return map[string]any{"task_id": taskID}, nil
}
