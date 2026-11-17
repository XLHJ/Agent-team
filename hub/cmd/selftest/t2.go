package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t2(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	send(ctx, c, map[string]any{"jsonrpc": "2.0", "protocol_version": "0.1", "method": "plugin.heartbeat", "id": 9, "params": map[string]any{"instance_id": "x", "timestamp": "2026-11-13T00:00:00Z"}})
	m, err := read(ctx, c)
	if err != nil {
		fail("T2 pre-register reject", "closed without frame")
		return
	}
	if errCode(m) == "-32600" {
		pass("T2 pre-register reject")
	} else {
		fail("T2 pre-register reject", errCode(m))
	}
	if err := waitClose(ctx, c); err != nil {
		pass("T2 connection close")
	} else {
		fail("T2 connection close", "still open")
	}
}
