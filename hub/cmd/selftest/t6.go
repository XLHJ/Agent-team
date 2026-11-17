package main

import (
	"context"
	"time"

	"nhooyr.io/websocket"
)

func t6(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-hb", []string{"observe"}, "0.1")
	_, _ = read(ctx, c)
	_, _ = read(ctx, c)

	var before []map[string]any
	getJSON("/registry", &before)
	t0 := time.Now().Add(-time.Second)
	send(ctx, c, map[string]any{"jsonrpc": "2.0", "protocol_version": "0.1", "method": "plugin.heartbeat", "id": 2, "params": map[string]any{"instance_id": "inst-hb", "timestamp": "2026-11-13T00:00:00Z"}})
	m, err := read(ctx, c)
	if err == nil && errCode(m) == "" && m["result"] != nil {
		pass("T6 heartbeat result")
	} else {
		fail("T6 heartbeat result", err)
	}
	var after []map[string]any
	getJSON("/registry", &after)
	for _, in := range after {
		if in["instance_id"] == "inst-hb" {
			if ts, ok := in["last_heartbeat_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, ts); err == nil && t.After(t0) {
					pass("T6 heartbeat persisted")
					return
				}
			}
		}
	}
	fail("T6 heartbeat persisted")
}
