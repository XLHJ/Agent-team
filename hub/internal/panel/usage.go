package panel

import (
	"encoding/json"
	"time"
)

// usageSummary serves panel.usage.summary: (model, source)-grouped usage
// aggregates for a time window. window: "24h" | "7d" | "31d" (also accepts
// explicit since_ms/until_ms). 24h reads hourly buckets, 7d/31d daily ones.
func (h *Handler) usageSummary(params json.RawMessage) (any, *rpcError) {
	if h.d.Store == nil {
		return nil, bizErr("USAGE_DISABLED", "usage store not configured")
	}
	// Fold any not-yet-aggregated detail rows in before querying; the
	// watermark makes this incremental and idempotent, so dashboards are
	// always current without waiting for the periodic sweep.
	if _, err := h.d.Store.UsageAggregate(); err != nil {
		return nil, bizErr("USAGE_QUERY_FAILED", err.Error())
	}
	var p struct {
		Window  string `json:"window"`
		SinceMs int64  `json:"since_ms"`
		UntilMs int64  `json:"until_ms"`
	}
	_ = json.Unmarshal(params, &p)
	now := time.Now().UnixMilli()
	if p.UntilMs <= 0 {
		p.UntilMs = now
	}
	var table string
	switch p.Window {
	case "24h":
		table = "usage_stats_hourly"
		if p.SinceMs <= 0 {
			p.SinceMs = now - 24*3600*1000
		}
	case "7d", "31d", "":
		if p.Window == "" {
			p.Window = "7d"
		}
		table = "usage_stats_daily"
		if p.SinceMs <= 0 {
			days := int64(7)
			if p.Window == "31d" {
				days = 31
			}
			p.SinceMs = now - days*24*3600*1000
		}
	default:
		return nil, bizErr("USAGE_BAD_WINDOW", "window must be one of 24h, 7d, 31d")
	}
	rows, err := h.d.Store.UsageSummary(table, p.SinceMs)
	if err != nil {
		return nil, bizErr("USAGE_QUERY_FAILED", err.Error())
	}
	// Totals for the same window.
	var in, out, calls int64
	for _, r := range rows {
		in += r.InputTokens
		out += r.OutputTokens
		calls += r.Calls
	}
	return map[string]any{
		"window":     p.Window,
		"since_ms":   p.SinceMs,
		"until_ms":   p.UntilMs,
		"rows":       rows,
		"totals":     map[string]any{"calls": calls, "input_tokens": in, "output_tokens": out},
		"updated_at": now,
	}, nil
}

// usageLogs serves panel.usage.logs: recent raw detail rows (history view).
func (h *Handler) usageLogs(params json.RawMessage) (any, *rpcError) {
	if h.d.Store == nil {
		return nil, bizErr("USAGE_DISABLED", "usage store not configured")
	}
	var p struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)
	items, err := h.d.Store.UsageLogsList(p.Limit)
	if err != nil {
		return nil, bizErr("USAGE_QUERY_FAILED", err.Error())
	}
	return map[string]any{"items": items}, nil
}
