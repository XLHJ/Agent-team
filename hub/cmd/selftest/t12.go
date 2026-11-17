package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t12(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	send(ctx, c, map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1", "method": "plugin.register", "id": 7,
		"params": map[string]any{"agent_type": "test"},
	})
	m, err := read(ctx, c)
	if err != nil {
		fail("T12 schema error", err)
		return
	}
	if errCode(m) == "-32602" || errCode(m) == "" && m["error"] != nil {
		if e, ok := m["error"].(map[string]any); ok {
			if d, ok := e["data"].(map[string]any); ok && d["field"] != nil {
				pass("T12 schema error with field")
				return
			}
		}
	}
	fail("T12 schema error with field", m)
}
