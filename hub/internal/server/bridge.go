package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"agent-control-plane/hub/internal/rpc"
	"agent-control-plane/hub/internal/tasks"
)

// Dify / external HTTP bridge.
//
// Dify workflows call external HTTP APIs via HTTP-request nodes or custom
// tools, so these endpoints let a Dify workflow allocate work to agents
// registered in the panel:
//
//	GET  /bridge/v1/agents            - list connected, dispatch-capable agents
//	POST /bridge/v1/dispatch          - dispatch a task (sync wait or async)
//	GET  /bridge/v1/tasks/{id}        - poll task status/result (async mode)
//
// Auth: same bootstrap token as the rest of the guarded endpoints
// (X-Hub-Token header or ?token= query, matching Dify custom-tool auth).

type bridgeDispatchReq struct {
	InstanceID string `json:"instance_id"`
	Prompt     string `json:"prompt"`
	TimeoutMS  int    `json:"timeout_ms"`
	Wait       bool   `json:"wait"`
	WaitSec    int    `json:"wait_timeout_s"`
}

func (s *Server) bridgeDispatch(w http.ResponseWriter, r *http.Request) {
	var q bridgeDispatchReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil || q.InstanceID == "" || q.Prompt == "" {
		writeJSON(w, 400, map[string]string{"error": "instance_id and prompt are required"})
		return
	}
	if !s.Reg.Has(q.InstanceID) {
		writeJSON(w, 404, map[string]string{"error": "INSTANCE_NOT_FOUND"})
		return
	}
	if !s.Reg.HasCapability(q.InstanceID, "task.dispatch") {
		writeJSON(w, 403, map[string]string{"error": rpc.BusCapabilityNotDeclared})
		return
	}
	if q.TimeoutMS <= 0 {
		q.TimeoutMS = 300000
	}
	taskID := fmt.Sprintf("bridge-%x", time.Now().UnixNano())
	s.Tasks.Dispatch(taskID, q.InstanceID, q.Prompt, q.TimeoutMS)
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1", "method": "task.dispatch",
		"params": map[string]any{"task_id": taskID, "prompt": q.Prompt, "timeout_ms": q.TimeoutMS},
	}
	if !s.Pool.SendTo(r.Context(), q.InstanceID, note) {
		_ = s.Tasks.Fail(taskID, "DISPATCH_FAILED", "connection unavailable", "")
		writeJSON(w, 502, map[string]string{"error": "DISPATCH_FAILED"})
		return
	}
	if !q.Wait {
		writeJSON(w, 200, map[string]any{"task_id": taskID, "state": string(tasks.Dispatched), "ok": true})
		return
	}
	// Sync mode: block until terminal state. Dify HTTP nodes have their own
	// (default 100s) timeout, so callers should keep wait_timeout_s below it.
	limit := q.WaitSec
	if limit <= 0 {
		limit = 90
	}
	deadline := time.Now().Add(time.Duration(limit) * time.Second)
	for {
		t, ok := s.Tasks.Get(taskID)
		if ok && isTerminalTask(t.State) {
			writeJSON(w, 200, bridgeTaskView(t))
			return
		}
		if time.Now().After(deadline) {
			// Not terminal yet: hand back the task_id so the workflow can
			// switch to polling instead of failing outright.
			writeJSON(w, 200, map[string]any{
				"task_id": taskID, "state": t.State, "ok": false,
				"timed_out": true, "message": "still running; poll /bridge/v1/tasks/" + taskID,
			})
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Server) bridgeTask(w http.ResponseWriter, r *http.Request) {
	t, ok := s.Tasks.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "TASK_NOT_FOUND"})
		return
	}
	writeJSON(w, 200, bridgeTaskView(t))
}

func (s *Server) bridgeAgents(w http.ResponseWriter, _ *http.Request) {
	type agentRow struct {
		InstanceID   string   `json:"instance_id"`
		AgentType    string   `json:"agent_type"`
		ConnStatus   string   `json:"conn_status"`
		Capabilities []string `json:"capabilities"`
	}
	out := []agentRow{}
	for _, in := range s.Reg.Snapshot() {
		dispatchable := false
		for _, c := range in.Capabilities {
			if c == "task.dispatch" {
				dispatchable = true
				break
			}
		}
		if !dispatchable {
			continue
		}
		out = append(out, agentRow{
			InstanceID: in.InstanceID, AgentType: in.AgentType,
			ConnStatus: in.ConnStatus, Capabilities: in.Capabilities,
		})
	}
	writeJSON(w, 200, map[string]any{"agents": out, "count": len(out)})
}

func isTerminalTask(st tasks.State) bool {
	return st == tasks.Completed || st == tasks.Failed || st == tasks.Canceled
}

func bridgeTaskView(t tasks.Task) map[string]any {
	ok := t.State == tasks.Completed
	v := map[string]any{
		"task_id": t.TaskID, "instance_id": t.InstanceID, "state": string(t.State), "ok": ok,
	}
	if t.Report != "" {
		v["report"] = t.Report
	}
	if t.RawOutput != "" {
		v["raw_output"] = t.RawOutput
	}
	if len(t.Artifacts) > 0 {
		v["artifacts"] = t.Artifacts
	}
	if t.ErrorCode != "" {
		v["error_code"] = t.ErrorCode
		v["error_message"] = t.ErrorMessage
	}
	return v
}
