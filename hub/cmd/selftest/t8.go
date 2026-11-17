package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t8(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-task", []string{"observe", "task.dispatch"}, "0.1")
	_, _ = read(ctx, c)
	_, _ = read(ctx, c)

	code, _ := postJSON("/selftest/tasks/dispatch", map[string]any{"instance_id": "inst-task", "task_id": "t8-1", "prompt": "do", "timeout_ms": 1000})
	if code != 200 {
		fail("T8 dispatch", code)
		return
	}
	_, _ = read(ctx, c) // task.dispatch notification

	bsend(ctx, c, "task.progress", 3, map[string]any{"task_id": "t8-1", "progress": 50.0, "step": "half"})
	_, _ = read(ctx, c) // await progress ack before polling
	var first map[string]any
	getJSON("/tasks/t8-1", &first)
	if first["state"] == "running" {
		pass("T8 running")
	} else {
		fail("T8 running", first["state"])
	}

	bsend(ctx, c, "task.completed", 4, map[string]any{"task_id": "t8-1", "raw_output": "done"})
	_, _ = read(ctx, c) // await completed ack before polling
	var second map[string]any
	getJSON("/tasks/t8-1", &second)
	if second["state"] == "completed" {
		pass("T8 completed")
	} else {
		fail("T8 completed", second["state"])
	}
}
