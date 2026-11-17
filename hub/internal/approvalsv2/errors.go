package approvalsv2

import "errors"

// Errors surfaced to the panel over JSON-RPC.
var (
	ErrNotFound    = errors.New("approval not found")
	ErrAlreadyDone = errors.New("approval already resolved")
	ErrBadDecision = errors.New("decision must be approve/deny/modify")
	ErrBadModify   = errors.New("modified_params must be a JSON object")
)
