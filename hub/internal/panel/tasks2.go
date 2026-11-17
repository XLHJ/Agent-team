package panel

import (
	"encoding/json"
)

func (h *Handler) taskGet(params json.RawMessage) (any, *rpcError) {
	var p struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(params, &p)
	t, ok := h.d.Tasks.Get(p.TaskID)
	if !ok {
		return nil, bizErr("TASK_NOT_FOUND", "no such task")
	}
	return t, nil
}
