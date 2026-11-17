package panel

import (
	"context"
	"encoding/json"

	"agent-control-plane/hub/internal/approvalsv2"
)

func (h *Handler) approvalsRespond(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		ApprovalID string          `json:"approval_id"`
		Decision   string          `json:"decision"`
		Modified   json.RawMessage `json:"modified_params"`
		Reason     string          `json:"reason"`
		Exempt     bool            `json:"exempt"`
	}
	_ = json.Unmarshal(params, &p)
	a, err := h.d.Appr.Resolve(p.ApprovalID, p.Decision, string(p.Modified), p.Reason, p.Exempt)
	switch err {
	case nil:
	case approvalsv2.ErrNotFound:
		return nil, bizErr("APPROVAL_NOT_FOUND", "no such approval")
	case approvalsv2.ErrAlreadyDone:
		return nil, bizErr("APPROVAL_ALREADY_RESOLVED", "approval already resolved")
	default:
		return nil, bizErr("INVALID_DECISION", err.Error())
	}
	rp := map[string]any{"approval_id": a.ApprovalID, "decision": a.Decision.String}
	if a.Reason.Valid {
		rp["reason"] = a.Reason.String
	}
	if a.ModifiedParams.Valid {
		rp["modified_params"] = json.RawMessage(a.ModifiedParams.String)
	}
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1",
		"method": "approval.respond", "params": rp,
	}
	if !h.d.Pool.SendTo(ctx, a.InstanceID, note) {
		return map[string]any{"approval_id": a.ApprovalID, "delivery": "failed"}, nil
	}
	return map[string]any{"approval_id": a.ApprovalID}, nil
}
