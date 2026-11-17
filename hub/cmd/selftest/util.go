package main

import (
	"context"

	"nhooyr.io/websocket"
)

func bsend(ctx context.Context, c *websocket.Conn, method string, id int, params any) {
	send(ctx, c, map[string]any{"jsonrpc": "2.0", "protocol_version": "0.1", "method": method, "id": id, "params": params})
}
