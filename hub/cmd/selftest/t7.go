package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"nhooyr.io/websocket"
)

func postJSON(p string, body any) (int, map[string]any) {
	b, _ := json.Marshal(body)
	resp, err := http.Post(url(p), "application/json", bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

func t7(ctx context.Context) {
	c := dial(ctx)
	defer c.Close(websocket.StatusNormalClosure, "")
	register(ctx, c, "inst-observe-only", []string{"observe"}, "0.1")
	_, _ = read(ctx, c)
	_, _ = read(ctx, c)
	code, m := postJSON("/selftest/tasks/dispatch", map[string]any{"instance_id": "inst-observe-only", "task_id": "t7-1", "prompt": "x", "timeout_ms": 1000})
	if code == 403 && m["error"] == "CAPABILITY_NOT_DECLARED" {
		pass("T7 capability guard")
	} else {
		fail("T7 capability guard", code, m["error"])
	}
}
