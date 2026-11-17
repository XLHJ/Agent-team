package panel

import (
	"encoding/json"
	"time"

	"agent-control-plane/hub/internal/tasks"
)

func (h *Handler) tasksList(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Status     string `json:"status"`
		InstanceID string `json:"instance_id"`
		Since      string `json:"since"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)
	var since time.Time
	if p.Since != "" {
		if t, err := time.Parse(time.RFC3339Nano, p.Since); err == nil {
			since = t
		}
	}
	list := h.d.Tasks.List(p.InstanceID, since)
	if p.Status != "" {
		var f []tasks.Task
		for _, t := range list {
			if string(t.State) == p.Status {
				f = append(f, t)
			}
		}
		list = f
	}
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 50
	}
	next := ""
	if len(list) > p.Limit {
		next = list[p.Limit-1].TaskID
		list = list[:p.Limit]
	}
	return map[string]any{"tasks": list, "next_cursor": next}, nil
}
