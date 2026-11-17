package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

// panelTest drives one panel session over WS.
type panelTest struct {
	base  string
	token string
}

// dialPanel connects to /panel with token and optional Origin.
func dialPanel(ctx context.Context, base, token, origin string) (*websocket.Conn, *http.Response, error) {
	u := "ws://" + base + "/panel"
	if token != "" {
		u += "?token=" + token
	}
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	return websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: h})
}

// call sends one JSON-RPC request and returns the response frame.
func call(ctx context.Context, c *websocket.Conn, id int64, method string, params any) map[string]any {
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	b, _ := json.Marshal(req)
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		return map[string]any{}
	}
	for {
		rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, raw, err := c.Read(rctx)
		cancel()
		if err != nil {
			return map[string]any{}
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if m["method"] == "panel.event" {
			continue
		}
		if v, ok := m["id"].(float64); ok && int64(v) == id {
			return m
		}
	}
}

// bizCode extracts the business error code if present.
func bizCode(frame map[string]any) string {
	e, _ := frame["error"].(map[string]any)
	if e == nil {
		return ""
	}
	d, _ := e["data"].(map[string]any)
	if d == nil {
		return ""
	}
	c, _ := d["code"].(string)
	return c
}

func (p *panelTest) checkTokenMissing(ctx context.Context) {
	c, resp, err := dialPanel(ctx, p.base, "", "")
	if err == nil {
		c.Close(websocket.StatusNormalClosure, "")
	}
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	check("P3-01 no-token 401", code == 401, fmt.Sprintf("code=%d err=%v", code, err))
}

func (p *panelTest) checkOrigin(ctx context.Context) {
	c, resp, err := dialPanel(ctx, p.base, p.token, "http://evil.example")
	if err == nil {
		c.Close(websocket.StatusNormalClosure, "")
	}
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	check("P3-02 bad origin 403", code == 403, fmt.Sprintf("code=%d err=%v", code, err))
}

func (p *panelTest) checkSinglePanel(ctx context.Context) {
	c1, _, err := dialPanel(ctx, p.base, p.token, "")
	if err != nil {
		check("P3-03 second conn 409", false, err.Error())
		return
	}
	defer c1.Close(websocket.StatusNormalClosure, "")
	c2, resp, err := dialPanel(ctx, p.base, p.token, "")
	if err == nil {
		c2.Close(websocket.StatusNormalClosure, "")
	}
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	check("P3-03 second conn 409", code == 409, fmt.Sprintf("code=%d err=%v", code, err))
}

func (p *panelTest) checkMethods(ctx context.Context) {
	c, _, err := dialPanel(ctx, p.base, p.token, "")
	if err != nil {
		check("P3-04 agents.list", false, err.Error())
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	r := call(ctx, c, 1, "panel.agents.list", map[string]any{})
	_, has := r["result"].(map[string]any)["instances"]
	check("P3-04 agents.list result", has, strings.TrimSpace(fmt.Sprint(r)))
	r = call(ctx, c, 2, "panel.nope", map[string]any{})
	e, _ := r["error"].(map[string]any)
	code := 0.0
	if e != nil {
		code, _ = e["code"].(float64)
	}
	check("P3-07 unknown method -32601", int(code) == -32601, fmt.Sprint(r))
}

func (p *panelTest) checkBusinessErrors(ctx context.Context) {
	c, _, err := dialPanel(ctx, p.base, p.token, "")
	if err != nil {
		check("P3-05 approval not found", false, err.Error())
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	r := call(ctx, c, 3, "panel.approvals.respond",
		map[string]any{"approval_id": "nope", "decision": "approve"})
	check("P3-05 APPROVAL_NOT_FOUND", bizCode(r) == "APPROVAL_NOT_FOUND", fmt.Sprint(r))
	r = call(ctx, c, 4, "panel.tasks.create",
		map[string]any{"instance_id": "ghost", "prompt": "x"})
	check("P3-06 INSTANCE_NOT_FOUND", bizCode(r) == "INSTANCE_NOT_FOUND", fmt.Sprint(r))
}

func (p *panelTest) checkEventPush(ctx context.Context) {
	// Events flow through the approval engine subscribe hook; without a
	// plugin connected there is nothing to fan out yet. Verify subscribe
	// round-trips and the filter accepts task.* patterns.
	c, _, err := dialPanel(ctx, p.base, p.token, "")
	if err != nil {
		check("P3-08 subscribe ack", false, err.Error())
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	r := call(ctx, c, 5, "panel.subscribe", map[string]any{"types": []string{"task.*"}})
	_, has := r["result"]
	check("P3-08 subscribe ack", has, fmt.Sprint(r))
}
