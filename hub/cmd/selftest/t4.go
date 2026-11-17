package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t4(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-badver", []string{"observe"}, "1.0")
	m, err := read(ctx, c)
	if err != nil {
		fail("T4 version mismatch", "closed without frame")
		return
	}
	if errCode(m) == "PROTOCOL_VERSION_MISMATCH" {
		pass("T4 version mismatch")
	} else {
		fail("T4 version mismatch", errCode(m))
	}
	if err := waitClose(ctx, c); err != nil {
		pass("T4 close after mismatch")
	} else {
		fail("T4 close after mismatch")
	}
}
