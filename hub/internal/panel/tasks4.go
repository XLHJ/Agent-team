package panel

import (
	"context"
	"encoding/json"

	"agent-control-plane/hub/internal/tasks"
)

func (h *Handler) taskCancel(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(params, &p)
	t, ok := h.d.Tasks.Get(p.TaskID)
	if !ok {
		return nil, bizErr("TASK_NOT_FOUND", "no such task")
	}
	switch t.State {
	case tasks.Completed, tasks.Failed, tasks.Canceled:
		return nil, bizErr("TASK_ALREADY_TERMINAL", "task is terminal")
	}
	_ = h.d.Tasks.Cancel(p.TaskID)
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1",
		"method": "task.cancel",
		"params": map[string]any{"task_id": p.TaskID, "reason": p.Reason},
	}
	_ = h.d.Pool.SendTo(ctx, t.InstanceID, note)
	h.d.Bus.Publish(SerEvent("task.progress", map[string]any{
		"task_id": p.TaskID, "state": "canceled",
	}))
	return map[string]any{}, nil
}
