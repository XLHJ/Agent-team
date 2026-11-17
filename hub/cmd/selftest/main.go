// Selftest runs the T1-T12 protocol checklist against an in-process hub.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"nhooyr.io/websocket"

	"agent-control-plane/hub/internal/approvalsv2"
	"agent-control-plane/hub/internal/registry"
	"agent-control-plane/hub/internal/rpc"
	"agent-control-plane/hub/internal/server"
	"agent-control-plane/hub/internal/store"
	"agent-control-plane/hub/internal/tasks"
	"agent-control-plane/hub/internal/ws"
)

var (
	baseURL string
	wsURL   string
	fails   int
)

func url(p string) string { return baseURL + p }

func getJSON(p string, out any) int {
	resp, err := http.Get(url(p))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, out)
	return resp.StatusCode
}

func dial(ctx context.Context) *websocket.Conn {
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		panic(err)
	}
	return c
}

func send(ctx context.Context, c *websocket.Conn, v any) {
	b, _ := json.Marshal(v)
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		panic(err)
	}
}

func read(ctx context.Context, c *websocket.Conn) (map[string]any, error) {
	rctx, cancel := context.WithTimeout(ctx, 3e9)
	defer cancel()
	_, b, err := c.Read(rctx)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m, nil
}

func errCode(m map[string]any) string {
	e, ok := m["error"].(map[string]any)
	if !ok {
		return ""
	}
	if d, ok := e["data"].(map[string]any); ok {
		if c, ok := d["code"].(string); ok {
			return c
		}
	}
	if n, ok := e["code"].(float64); ok {
		return fmt.Sprintf("%d", int(n))
	}
	return ""
}

func main() {
	schemas := "../protocol/schemas"
	if len(os.Args) > 1 {
		schemas = os.Args[1]
	}
	lg := log.New(io.Discard, "", 0)
	val, err := rpc.NewValidator(schemas)
	if err != nil {
		panic(err)
	}
	reg := registry.New()
	tsk := tasks.New()
	st, stErr := store.Open(":memory:")
	if stErr != nil {
		panic(stErr)
	}
	defer st.Close()
	if err := st.Schema(); err != nil {
		panic(err)
	}
	app := approvalsv2.New(st, lg)
	pool := ws.NewPool()
	rt := rpc.NewRouter(rpc.Deps{Reg: reg, Tasks: tsk, Appr: app, Pool: pool, Val: val, Log: lg})
	srv := server.New(reg, tsk, app, pool, rt, lg, nil)
	hs := httptest.NewServer(srv.Routes())
	defer hs.Close()
	baseURL = hs.URL
	wsURL = "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"

	ctx := context.Background()
	t1()
	t2(ctx)
	t3(ctx)
	t4(ctx)
	t5(ctx)
	t6(ctx)
	t7(ctx)
	t8(ctx)
	t9(ctx)
	t10(ctx)
	t11(ctx)
	t12(ctx)

	fmt.Printf("TOTAL fails=%d\n", fails)
	if fails > 0 {
		os.Exit(1)
	}
}
