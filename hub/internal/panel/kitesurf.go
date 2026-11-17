package panel

import (
	"encoding/json"
)

// Kitesurf bridge configuration (endpoint stored in app_settings, used by
// the hub-side /kitesurf/v1/* bridge endpoints that all agents call).

// kitesurfGet serves panel.kitesurf.get.
func (h *Handler) kitesurfGet(_ json.RawMessage) (any, *rpcError) {
	if h.d.Store == nil {
		return nil, bizErr("STORE_DISABLED", "settings store not configured")
	}
	ep, _ := h.d.Store.SettingGet("kitesurf.ws_endpoint")
	return map[string]any{"endpoint": ep, "configured": ep != ""}, nil
}

// kitesurfSet serves panel.kitesurf.set {endpoint} (empty clears it).
func (h *Handler) kitesurfSet(params json.RawMessage) (any, *rpcError) {
	if h.d.Store == nil {
		return nil, bizErr("STORE_DISABLED", "settings store not configured")
	}
	var p struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.Unmarshal(params, &p)
	if err := h.d.Store.SettingSet("kitesurf.ws_endpoint", p.Endpoint); err != nil {
		return nil, bizErr("STORE_WRITE_FAILED", err.Error())
	}
	return map[string]any{"ok": true, "endpoint": p.Endpoint}, nil
}
