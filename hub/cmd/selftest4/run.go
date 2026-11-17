package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p
}

func waitReady(url string) {
	for i := 0; i < 200; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println("hub never became ready")
	os.Exit(2)
}

func httpGet(url, token string) string {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Hub-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func httpPost(url, token string, payload any) string {
	var r io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		r = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest("POST", url, r)
	req.Header.Set("X-Hub-Token", token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func run(hubBin, runnerBin, agentBin string) {
	dir, _ := os.MkdirTemp("", "acp-st4-")
	defer os.RemoveAll(dir)
	port := freePort()
	token := "selftest4-token"
	hub := exec.Command(hubBin,
		"-addr", "127.0.0.1:"+strconv.Itoa(port),
		"-token", token, "-state-dir", dir,
		"-harness-bin", runnerBin,
		"-schemas", "../../../protocol/schemas",
		"-signatures", "../../../harness-sdk/detect/signatures",
		"-workspace", "../../../",
	)
	hub.Stdout = os.Stdout
	hub.Stderr = os.Stderr
	if err := hub.Start(); err != nil {
		fmt.Println("hub start:", err)
		os.Exit(2)
	}
	defer func() { _ = hub.Process.Kill() }()
	waitReady("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
	_ = agentBin
	base := "127.0.0.1:" + strconv.Itoa(port)
	p4 := &wfTest{base: base, token: token}
	ctx := context.Background()
	if err := p4.e2e(ctx); err != nil {
		check("W e2e flow", false, err.Error())
	}
}

type wfTest struct {
	base  string
	token string
}

// e2e drives the full Phase-5 flow over the plugin WS and the panel WS.
func (p *wfTest) e2e(ctx context.Context) error {
	plugin, _, err := websocket.Dial(ctx, "ws://"+p.base+"/ws?token="+p.token, nil)
	if err != nil {
		return err
	}
	defer plugin.Close(websocket.StatusNormalClosure, "")
	inst := p.register(ctx, plugin)
	if inst == "" {
		return fmt.Errorf("register failed")
	}
	check("W2 plugin registered", true, inst)
	panel, _, err := websocket.Dial(ctx, "ws://"+p.base+"/panel?token="+p.token, nil)
	if err != nil {
		return err
	}
	defer panel.Close(websocket.StatusNormalClosure, "")
	wfID := "wf-test-1"
	save := p.call(ctx, panel, 1, "panel.workflows.save", map[string]any{
		"workflow": map[string]any{"workflow_id": wfID, "name": "test chain",
			"status": "active"},
		"stages": []map[string]any{
			{"stage_id": "s1", "task_prompt": "step one", "report_prompt": "report one"},
			{"stage_id": "s2", "task_prompt": "step two", "report_prompt": "report two"},
		},
	})
	_, ok := save["result"]
	check("W1 workflow saved", ok, fmt.Sprint(save))
	start := p.call(ctx, panel, 2, "panel.workflows.start",
		map[string]any{"workflow_id": wfID})
	res, _ := start["result"].(map[string]any)
	runID, _ := res["run_id"].(string)
	check("W2 workflow started", runID != "", fmt.Sprint(start))
	// The plugin must receive the stage 0 dispatch (skip other notices).
	var frame map[string]any
	for i := 0; i < 5; i++ {
		frame = p.read(ctx, plugin)
		if frame["method"] == "task.dispatch" {
			break
		}
	}
	okDispatch := frame["method"] == "task.dispatch"
	check("W3 stage 0 dispatched", okDispatch, fmt.Sprint(frame))
	// Complete stage 0 with a report; stage 1 must carry the handoff.
	p.send(ctx, plugin, "task.completed", map[string]any{
		"task_id": "wf-" + runID + "-0", "raw_output": "out0",
		"report": "stage zero report",
	})
	time.Sleep(300 * time.Millisecond)
	var frame2 map[string]any
	for i := 0; i < 5; i++ {
		frame2 = p.read(ctx, plugin)
		if frame2["method"] == "task.dispatch" {
			break
		}
	}
	params, _ := frame2["params"].(map[string]any)
	input, _ := params["input_context"].(string)
	handoff := strings.Contains(input, "上一阶段进度报告") &&
		strings.Contains(input, "stage zero report") &&
		strings.Contains(input, "step two")
	check("W4 handoff format", handoff, fmt.Sprint(frame2))
	// Stage 1 completes without a report: run must fail REPORT_MISSING.
	p.send(ctx, plugin, "task.completed", map[string]any{
		"task_id": "wf-" + runID + "-1", "raw_output": "out1", "report": "",
	})
	time.Sleep(300 * time.Millisecond)
	rg := p.call(ctx, panel, 3, "panel.workflows.run",
		map[string]any{"run_id": runID})
	res2, _ := rg["result"].(map[string]any)
	runObj, _ := res2["run"].(map[string]any)
	check("W5 report missing fails run",
		runObj != nil && runObj["status"] == "failed" &&
			strings.Contains(fmt.Sprint(runObj["failure_reason"]), "REPORT_MISSING"),
		fmt.Sprint(runObj))
	// Export the chain, import it back; the copy must be a new id with
	// the same definition.
	ex := p.call(ctx, panel, 4, "panel.workflows.export",
		map[string]any{"workflow_id": wfID})
	exRes, _ := ex["result"].(map[string]any)
	chainObj, _ := exRes["chain"].(map[string]any)
	check("W6 chain exported",
		chainObj != nil && chainObj["version"] == "chain.v1" &&
			chainObj["name"] == "test chain",
		fmt.Sprint(ex))
	imp := p.call(ctx, panel, 5, "panel.workflows.import",
		map[string]any{"chain": chainObj})
	impRes, _ := imp["result"].(map[string]any)
	newID, _ := impRes["workflow_id"].(string)
	check("W7 chain imported", newID != "" && newID != wfID, fmt.Sprint(imp))
	if newID != "" {
		got := p.call(ctx, panel, 6, "panel.workflows.get",
			map[string]any{"workflow_id": newID})
		gRes, _ := got["result"].(map[string]any)
		gStages, _ := gRes["stages"].([]any)
		wObj, _ := gRes["workflow"].(map[string]any)
		okRoundtrip := wObj != nil && wObj["name"] == "test chain" &&
			wObj["status"] == "draft" && len(gStages) == 2
		check("W8 roundtrip definition", okRoundtrip, fmt.Sprint(got))
	}
	// Source preview backend: list, traversal rejection, file read.
	lst := p.call(ctx, panel, 7, "panel.files.list", map[string]any{"path": "docs"})
	lRes, _ := lst["result"].(map[string]any)
	entries, _ := lRes["entries"].([]any)
	foundDoc := false
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok && m["name"] == "phase5-acceptance.md" {
			foundDoc = true
		}
	}
	check("W9 files.list docs", foundDoc, fmt.Sprint(lst))
	trav := p.call(ctx, panel, 8, "panel.files.list", map[string]any{"path": "../harness-sdk"})
	_, hasResult := trav["result"]
	check("W10 traversal rejected", !hasResult && trav["error"] != nil, fmt.Sprint(trav))
	rd := p.call(ctx, panel, 9, "panel.files.read",
		map[string]any{"path": "hub/internal/files/files.go"})
	rdRes, _ := rd["result"].(map[string]any)
	file, _ := rdRes["file"].(map[string]any)
	check("W11 files.read go file",
		file != nil && file["language"] == "go" &&
			strings.Contains(fmt.Sprint(file["content"]), "sandboxed"),
		fmt.Sprint(rd))
	// Usage: task-level usage.report is persisted, aggregated on read and
	// surfaced via panel.usage.summary / panel.usage.logs.
	p.send(ctx, plugin, "usage.report", map[string]any{
		"task_id": "usage-task-1", "instance_id": "inst-selftest4",
		"calls": []map[string]any{
			{"model": "dsv4.1", "source": "official", "input_tokens": 100,
				"output_tokens": 200, "ttft_ms": 300, "tokens_per_s": 40.0, "duration_ms": 5000},
			{"model": "dsv4.1", "source": "official", "input_tokens": 50,
				"output_tokens": 100, "ttft_ms": 500, "tokens_per_s": 20.0, "duration_ms": 5000},
			{"model": "gpt-6-sol", "source": "relay-station-A", "input_tokens": 10,
				"output_tokens": 30, "ttft_ms": 100, "tokens_per_s": 60.0, "duration_ms": 500},
		},
	})
	time.Sleep(300 * time.Millisecond)
	us := p.call(ctx, panel, 10, "panel.usage.summary", map[string]any{"window": "24h"})
	uRes, _ := us["result"].(map[string]any)
	uRows, _ := uRes["rows"].([]any)
	okSummary := false
	for _, r := range uRows {
		m, _ := r.(map[string]any)
		if m["model"] == "dsv4.1" && m["source"] == "official" {
			okSummary = m["calls"] == float64(2) &&
				m["input_tokens"] == float64(150) &&
				m["output_tokens"] == float64(300)
		}
	}
	check("W12 usage summary grouped", okSummary, fmt.Sprint(us))
	// An invalid report (missing calls) must be rejected with an error reply.
	badReq := map[string]any{"jsonrpc": "2.0", "protocol_version": "0.1",
		"id": 777, "method": "usage.report", "params": map[string]any{"task_id": "x"}}
	bb, _ := json.Marshal(badReq)
	_ = plugin.Write(ctx, websocket.MessageText, bb)
	badErr := false
	for i := 0; i < 6; i++ {
		f := p.read(ctx, plugin)
		if v, ok := f["id"].(float64); ok && int64(v) == 777 && f["error"] != nil {
			badErr = true
			break
		}
	}
	check("W13 invalid usage rejected", badErr, "")
	ul := p.call(ctx, panel, 11, "panel.usage.logs", map[string]any{"limit": 10})
	lRes, _ = ul["result"].(map[string]any)
	lItems, _ := lRes["items"].([]any)
	check("W14 usage raw logs", len(lItems) == 3, fmt.Sprint(ul))

	// Dify bridge: agents list, async dispatch + polling, sync wait.
	hbase := "http://" + p.base
	ag := httpGet(hbase+"/bridge/v1/agents", p.token)
	check("W15 bridge agents", strings.Contains(ag, "inst-selftest4") &&
		strings.Contains(ag, "task.dispatch"), ag)
	// Simulated agent: answer bridge-dispatched tasks so they run to
	// completion while the main goroutine polls over HTTP.
	agentDone := make(chan struct{})
	defer close(agentDone)
	go func() {
		for {
			select {
			case <-agentDone:
				return
			default:
			}
			f := p.read(ctx, plugin)
			if f["method"] == "task.dispatch" {
				m, _ := f["params"].(map[string]any)
				tid, _ := m["task_id"].(string)
				p.send(ctx, plugin, "task.progress", map[string]any{"task_id": tid})
				p.send(ctx, plugin, "task.completed", map[string]any{
					"task_id": tid, "raw_output": "bridge ok", "report": "bridge report",
				})
			}
		}
	}()

	dr := httpPost(hbase+"/bridge/v1/dispatch", "selftest4-token", map[string]any{
		"instance_id": "inst-selftest4", "prompt": "bridge async task", "wait": false,
	})
	dm := map[string]any{}
	_ = json.Unmarshal([]byte(dr), &dm)
	bid, _ := dm["task_id"].(string)
	check("W16 bridge async dispatch", dm["ok"] == true && bid != "", dr)
	bState := ""
	for i := 0; i < 40; i++ {
		st := httpGet(hbase+"/bridge/v1/tasks/"+bid, "selftest4-token")
		bm := map[string]any{}
		_ = json.Unmarshal([]byte(st), &bm)
		bState, _ = bm["state"].(string)
		if bState == "completed" || bState == "failed" {
			check("W17 bridge async result", bState == "completed" && bm["ok"] == true, st)
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if bState != "completed" && bState != "failed" {
		check("W17 bridge async result", false, "never terminal")
	}
	sr := httpPost(hbase+"/bridge/v1/dispatch", "selftest4-token", map[string]any{
		"instance_id": "inst-selftest4", "prompt": "bridge sync task", "wait": true, "wait_timeout_s": 30,
	})
	sm := map[string]any{}
	_ = json.Unmarshal([]byte(sr), &sm)
	check("W18 bridge sync wait", sm["ok"] == true && sm["state"] == "completed", sr)
	// Kitesurf bridge config roundtrip (endpoint stored hub-side).
	p.call(ctx, panel, 13, "panel.kitesurf.set",
		map[string]any{"endpoint": "ws://127.0.0.1:8787/devtools/browser"})
	kg := p.call(ctx, panel, 14, "panel.kitesurf.get", map[string]any{})
	kRes, _ := kg["result"].(map[string]any)
	check("W19 kitesurf config", kRes != nil && kRes["configured"] == true &&
		strings.Contains(fmt.Sprint(kRes["endpoint"]), ":8787"), fmt.Sprint(kg))
	return nil
}

// register performs plugin.register and returns the instance id on success.
func (p *wfTest) register(ctx context.Context, c *websocket.Conn) string {
	p.send(ctx, c, "plugin.register", map[string]any{
		"instance_id": "inst-selftest4", "agent_type": "go-test",
		"protocol_version": "0.1",
		"agent_version":    "0.1",
		"capabilities":     []string{"task.dispatch", "report.generate"},
	})
	f := p.read(ctx, c)
	if f["error"] != nil {
		return ""
	}
	return "inst-selftest4"
}

// send writes one upstream request frame.
func (p *wfTest) send(ctx context.Context, c *websocket.Conn, method string, params map[string]any) {
	frame := map[string]any{"jsonrpc": "2.0", "protocol_version": "0.1",
		"id": time.Now().UnixNano(), "method": method, "params": params}
	b, _ := json.Marshal(frame)
	_ = c.Write(ctx, websocket.MessageText, b)
}

// read pulls the next frame (replies and notifications alike).
func (p *wfTest) read(ctx context.Context, c *websocket.Conn) map[string]any {
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, raw, err := c.Read(rctx)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

// call does one panel JSON-RPC round trip.
func (p *wfTest) call(ctx context.Context, c *websocket.Conn, id int64, method string, params map[string]any) map[string]any {
	frame := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	b, _ := json.Marshal(frame)
	_ = c.Write(ctx, websocket.MessageText, b)
	for {
		f := p.read(ctx, c)
		if v, ok := f["id"].(float64); ok && int64(v) == id {
			return f
		}
	}
}
