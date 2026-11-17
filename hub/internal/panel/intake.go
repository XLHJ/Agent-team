package panel

import "encoding/json"

func (h *Handler) intakeConfirm(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(params, &p)
	c, ok := h.d.Intake.Get(p.Name)
	if !ok {
		return nil, bizErr("CANDIDATE_NOT_FOUND", "no such candidate")
	}
	if len(c.Methods) == 0 {
		_ = h.d.Intake.SetState(p.Name, "failed", "no takeover method")
		return nil, bizErr("NO_METHOD", "candidate has no method")
	}
	if _, err := h.d.Take.Spawn(c.Signature, c.Path, c.Methods); err != nil {
		_ = h.d.Intake.SetState(p.Name, "failed", err.Error())
		return nil, bizErr("SPAWN_FAILED", err.Error())
	}
	_ = h.d.Intake.SetState(p.Name, "confirmed", "")
	return map[string]any{"name": p.Name, "state": "confirmed"}, nil
}

func (h *Handler) intakeIgnore(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(params, &p)
	if err := h.d.Intake.SetState(p.Name, "ignored", ""); err != nil {
		return nil, bizErr("CANDIDATE_NOT_FOUND", "no such candidate")
	}
	return map[string]any{"name": p.Name, "state": "ignored"}, nil
}
