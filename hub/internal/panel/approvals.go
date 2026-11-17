package panel

import (
	"encoding/json"
)

func (h *Handler) approvalsList(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Status     string `json:"status"`
		InstanceID string `json:"instance_id"`
		TaskID     string `json:"task_id"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)
	rows, err := h.d.Appr.List(p.Status, p.InstanceID, p.TaskID, p.Limit)
	if err != nil {
		return nil, bizErr("DB_ERROR", err.Error())
	}
	return map[string]any{"items": rows}, nil
}
