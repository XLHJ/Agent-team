// Package sidecar registers an out-of-process plugin. Sidecar is the fallback
// when the host exposes no embeddable extension point (Phase-2 5.2 order).
package sidecar

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"agent-control-plane/harness-sdk/validate"
	"agent-control-plane/harness-sdk/wire"

	"agent-control-plane/plugin-sdk/shared"
)

// AgentAdapter is what a Phase-3 adapter implements on the host side.
type AgentAdapter interface {
	// ApprovalMode returns native_forward or tool_hook.
	ApprovalMode() string
	// FormChoiceReason records why sidecar was chosen over embedded.
	FormChoiceReason() string
}

// RunSidecarPlugin starts a sidecar plugin for adapter.
func RunSidecarPlugin(ctx context.Context, cfg shared.Config, a AgentAdapter, h shared.Hooks, val *validate.Validator) (*shared.Plugin, error) {
	cfg.Form = "sidecar"
	cfg.ApprovalMode = a.ApprovalMode()
	cfg.Reason = a.FormChoiceReason()
	if cfg.InstanceID == "" {
		cfg.InstanceID = "plugin:" + cfg.AgentType + ":" + shortID()
	}
	if cfg.ApprovalMode != "" {
		cfg.Capabilities = append(cfg.Capabilities, wire.CapApproval)
	}
	return shared.Start(ctx, cfg, h, val)
}

func shortID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
