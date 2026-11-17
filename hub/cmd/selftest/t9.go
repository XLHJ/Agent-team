package main

import (
	"context"

	"nhooyr.io/websocket"
)

func t9(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-appr", []string{"observe", "task.dispatch", "approval"}, "0.1")
	_, _ = read(ctx, c)
	_, _ = read(ctx, c)

	postJSON("/selftest/tasks/dispatch", map[string]any{"instance_id": "inst-appr", "task_id": "t9-1", "prompt": "do", "timeout_ms": 1000})
	_, _ = read(ctx, c)

	bsend(ctx, c, "approval.requested", 5, map[string]any{
		"approval_id": "ap-1", "task_id": "t9-1", "instance_id": "inst-appr",
		"operation_type": "write_file", "risk_level": "medium", "context_summary": "write /tmp/x",
	})
	_, _ = read(ctx, c)

	var q []map[string]any
	getJSON("/approvals", &q)
	if len(q) == 1 && q[0]["status"] == "pending" {
		pass("T9 approval queued")
	} else {
		fail("T9 approval queued", len(q))
	}

	code, _ := postJSON("/selftest/approvals/respond", map[string]any{"approval_id": "ap-1", "decision": "approve", "reason": "ok"})
	if code == 200 {
		pass("T9 respond routed")
	} else {
		fail("T9 respond routed", code)
	}
	m, err := read(ctx, c)
	if err == nil && m["method"] == "approval.respond" {
		pass("T9 plugin received respond")
	} else {
		fail("T9 plugin received respond", err)
	}
	getJSON("/approvals", &q)
	if len(q) == 1 && q[0]["status"] == "resolved_approved" && q[0]["decision"] == "approve" {
		pass("T9 resolved state")
	} else {
		fail("T9 resolved state")
	}
}
