package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t5(ctx context.Context) {
	c1 := dial(ctx)
	defer c1.Close(websocket.StatusNormalClosure, "")
	register(ctx, c1, "inst-dup", []string{"observe"}, "0.1")
	_, _ = read(ctx, c1)
	_, _ = read(ctx, c1)

	c2 := dial(ctx)
	defer c2.Close(websocket.StatusNormalClosure, "")
	register(ctx, c2, "inst-dup", []string{"observe"}, "0.1")
	m, err := read(ctx, c2)
	if err != nil {
		fail("T5 id conflict", "closed without frame")
		return
	}
	if errCode(m) == "INSTANCE_ID_CONFLICT" {
		pass("T5 id conflict")
	} else {
		fail("T5 id conflict", errCode(m))
	}
}
