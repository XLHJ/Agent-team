package main

import (
	"context"
	"fmt"
	"time"

	"nhooyr.io/websocket"
)

func pass(name string) { fmt.Println("PASS", name) }

func fail(name string, args ...any) {
	fails++
	fmt.Println("FAIL", name, fmt.Sprint(args...))
}

func register(ctx context.Context, c *websocket.Conn, id string, caps []string, pv string) {
	send(ctx, c, map[string]any{
		"jsonrpc": "2.0", "protocol_version": pv, "method": "plugin.register", "id": 1,
		"params": map[string]any{
			"agent_type": "test", "instance_id": id,
			"protocol_version": pv, "capabilities": caps,
		},
	})
}

func waitClose(ctx context.Context, c *websocket.Conn) error {
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _, err := c.Read(rctx)
	return err
}
