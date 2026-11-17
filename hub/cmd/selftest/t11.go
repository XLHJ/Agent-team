package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t11(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	if err := c.Write(ctx, websocket.MessageText, []byte("not json at all")); err != nil {
		fail("T11 parse error", err)
		return
	}
	m, err := read(ctx, c)
	if err != nil {
		fail("T11 parse error", err)
		return
	}
	if errCode(m) == "-32700" {
		pass("T11 parse error")
	} else {
		fail("T11 parse error", errCode(m))
	}
}
