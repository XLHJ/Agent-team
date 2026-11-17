package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t3(ctx context.Context) string {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-1", []string{"observe", "task.dispatch"}, "0.1")
	m, err := read(ctx, c)
	if err != nil || errCode(m) != "" {
		fail("T3 register", err)
		return ""
	}
	n, _ := read(ctx, c)
	if n["method"] == "plugin.registered" {
		pass("T3 register confirmed")
	} else {
		fail("T3 plugin.registered", n["method"])
	}
	var snap []map[string]any
	getJSON("/registry", &snap)
	for _, in := range snap {
		if in["instance_id"] == "inst-1" {
			pass("T3 registry visible")
			return "inst-1"
		}
	}
	fail("T3 registry visible")
	return ""
}
