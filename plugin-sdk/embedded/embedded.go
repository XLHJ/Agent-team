// Package embedded registers a plugin that lives inside the Agent host process
// (Go hosts import it directly; Electron hosts use the TS client twin).
package embedded

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"agent-control-plane/harness-sdk/validate"
	"agent-control-plane/harness-sdk/wire"

	"agent-control-plane/plugin-sdk/shared"
)

// RegisterEmbeddedPlugin starts an in-host plugin. Reason is stored as the
// mandatory FormChoiceReason (Phase-2 risk 6).
func RegisterEmbeddedPlugin(ctx context.Context, cfg shared.Config, h shared.Hooks, reason string, val *validate.Validator) (*shared.Plugin, error) {
	cfg.Form = "embedded"
	cfg.Reason = reason
	if cfg.InstanceID == "" {
		cfg.InstanceID = "plugin:" + cfg.AgentType + ":" + shortID()
	}
	cfg.Capabilities = withDefaults(cfg.Capabilities)
	return shared.Start(ctx, cfg, h, val)
}

func withDefaults(caps []string) []string {
	out := append([]string{}, caps...)
	for _, c := range []string{wire.CapObserve, wire.CapTaskDispatch, wire.CapReportGenerate} {
		found := false
		for _, have := range out {
			if have == c {
				found = true
			}
		}
		if !found {
			out = append(out, c)
		}
	}
	return out
}

func shortID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
