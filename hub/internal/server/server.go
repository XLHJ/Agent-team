package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"nhooyr.io/websocket"

	"agent-control-plane/harness-sdk/detect"

	"agent-control-plane/hub/internal/approvalsv2"
	"agent-control-plane/hub/internal/intake"
	"agent-control-plane/hub/internal/panel"
	"agent-control-plane/hub/internal/registry"
	"agent-control-plane/hub/internal/rpc"
	"agent-control-plane/hub/internal/store"
	"agent-control-plane/hub/internal/takeover"
	"agent-control-plane/hub/internal/tasks"
	"agent-control-plane/hub/internal/ws"
)

// Server wires HTTP + WS handlers over the stores.
type Server struct {
	Reg    *registry.Registry
	Tasks  *tasks.Store
	Appr   *approvalsv2.Engine
	Pool   *ws.Pool
	Rt     *rpc.Router
	Log    *log.Logger
	Intake *intake.Manager
	Take   *takeover.Manager
	Sigs   *detect.Table
	Token  string
	Store  *store.DB
	Panel  *panel.Handler
	Bus    *panel.Bus
}

// New builds a Server (validator already constructed by caller).
func New(reg *registry.Registry, ts *tasks.Store, ap *approvalsv2.Engine, pool *ws.Pool, rt *rpc.Router, lg *log.Logger, st *store.DB) *Server {
	return &Server{Reg: reg, Tasks: ts, Appr: ap, Pool: pool, Rt: rt, Log: lg, Store: st}
}

// Routes builds the chi mux.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", s.health)
	r.Get("/registry", s.registrySnapshot)
	r.Get("/tasks/{id}", s.taskByID)
	r.Get("/approvals", s.approvalSnapshot)
	r.Post("/selftest/tasks/dispatch", s.selftestDispatch)
	r.Post("/selftest/approvals/respond", s.selftestRespond)
	r.Get("/ws", s.websocket)
	if s.Panel != nil {
		r.Get("/panel", s.Panel.Handle)
	}
	// Phase-2 intake / recovery surface (token-guarded).
	r.Get("/intake/candidates", s.guard(s.candidates))
	r.Post("/intake/rescan", s.guard(s.rescan))
	r.Post("/intake/confirm", s.guard(s.confirm))
	r.Post("/intake/ignore", s.guard(s.ignore))
	r.Get("/agents", s.guard(s.registrySnapshot))
	r.Post("/agents/{id}/disconnect", s.guard(s.disconnect))
	r.Post("/agents/{id}/retry", s.guard(s.retry))
	r.Get("/tasks", s.guard(s.tasksList))
	r.Get("/runners", s.guard(s.runners))
	// Kitesurf bridge (Cloudflare Workers browser engine via CDP).
	r.Get("/kitesurf/v1/status", s.guard(s.kitesurfStatus))
	r.Post("/kitesurf/v1/render", s.guard(s.kitesurfRender))
	// Dify / external HTTP bridge (token-guarded).
	r.Get("/bridge/v1/agents", s.guard(s.bridgeAgents))
	r.Post("/bridge/v1/dispatch", s.guard(s.bridgeDispatch))
	r.Get("/bridge/v1/tasks/{id}", s.guard(s.bridgeTask))
	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) registrySnapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Reg.Snapshot())
}

func (s *Server) approvalSnapshot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := s.Appr.List(q.Get("status"), q.Get("instance_id"), q.Get("task_id"), 500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "DB_ERROR"})
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) taskByID(w http.ResponseWriter, r *http.Request) {
	t, ok := s.Tasks.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "TASK_NOT_FOUND"})
		return
	}
	writeJSON(w, 200, t)
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	if s.Token != "" && r.URL.Query().Get("token") != s.Token {
		writeJSON(w, 401, map[string]string{"error": "UNAUTHORIZED"})
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	conn := &ws.Conn{C: c, LastSeen: time.Now()}
	s.readLoop(conn)
}

func (s *Server) readLoop(conn *ws.Conn) {
	ctx := context.Background()
	st := &rpc.ConnState{}
	defer func() {
		if conn.InstanceID != "" {
			s.Pool.RemoveIf(conn.InstanceID, conn)
			// Keep the registry record and non-terminal tasks untouched: the
			// recovering runner reports TASK_LOST_ON_RECONNECT for them.
			s.Reg.SetConnStatus(conn.InstanceID, registry.StatusUnreachable)
			s.Log.Printf("disconnected instance=%s", conn.InstanceID)
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}()
	for {
		_, raw, err := conn.C.Read(ctx)
		if err != nil {
			return
		}
		conn.LastSeen = time.Now()
		reply, closeAfter := s.Rt.Handle(ctx, conn, st, raw)
		if reply != nil {
			if werr := conn.WriteRaw(ctx, reply); werr != nil {
				return
			}
		}
		if st.PendingNotice != nil {
			if werr := conn.Send(ctx, st.PendingNotice); werr != nil {
				return
			}
			st.PendingNotice = nil
		}
		if closeAfter {
			return
		}
	}
}

// Sweep marks connections as UNREACHABLE after idle timeout.
func (s *Server) Sweep(ctx context.Context, idle time.Duration) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, in := range s.Reg.Snapshot() {
				if c, ok := s.Pool.Get(in.InstanceID); ok && time.Since(c.LastSeen) > idle {
					if in.ConnStatus != registry.StatusUnreachable {
						s.Reg.SetConnStatus(in.InstanceID, registry.StatusUnreachable)
						s.Log.Printf("instance=%s marked UNREACHABLE (idle)", in.InstanceID)
					}
				}
			}
		}
	}
}

type dispatchReq struct {
	InstanceID string `json:"instance_id"`
	TaskID     string `json:"task_id"`
	Prompt     string `json:"prompt"`
	TimeoutMS  int    `json:"timeout_ms"`
}

func (s *Server) selftestDispatch(w http.ResponseWriter, r *http.Request) {
	var q dispatchReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
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
	s.Tasks.Dispatch(q.TaskID, q.InstanceID, q.Prompt, q.TimeoutMS)
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1", "method": "task.dispatch",
		"params": map[string]any{"task_id": q.TaskID, "prompt": q.Prompt, "timeout_ms": q.TimeoutMS},
	}
	if !s.Pool.SendTo(r.Context(), q.InstanceID, note) {
		_ = s.Tasks.Fail(q.TaskID, "DISPATCH_FAILED", "connection unavailable", "")
		writeJSON(w, 502, map[string]string{"error": "DISPATCH_FAILED"})
		return
	}
	writeJSON(w, 200, map[string]string{"task_id": q.TaskID})
}

type respondReq struct {
	ApprovalID string          `json:"approval_id"`
	Decision   string          `json:"decision"`
	Modified   json.RawMessage `json:"modified_params"`
	Reason     string          `json:"reason"`
	Exempt     bool            `json:"exempt"`
}

func (s *Server) selftestRespond(w http.ResponseWriter, r *http.Request) {
	var q respondReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	a, err := s.Appr.Resolve(q.ApprovalID, q.Decision, string(q.Modified), q.Reason, q.Exempt)
	switch {
	case err == approvalsv2.ErrNotFound:
		writeJSON(w, 404, map[string]string{"error": rpc.BusApprovalNotFound})
		return
	case err == approvalsv2.ErrAlreadyDone:
		writeJSON(w, 409, map[string]string{"error": rpc.BusApprovalAlreadyResolved})
		return
	case err == approvalsv2.ErrBadModify || err == approvalsv2.ErrBadDecision:
		writeJSON(w, 400, map[string]string{"error": "INVALID_DECISION", "detail": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "DB_ERROR", "detail": err.Error()})
		return
	}
	params := map[string]any{
		"approval_id": a.ApprovalID, "decision": a.Decision.String, "reason": a.Reason.String,
	}
	if a.ModifiedParams.Valid {
		params["modified_params"] = json.RawMessage(a.ModifiedParams.String)
	}
	note := map[string]any{
		"jsonrpc": "2.0", "protocol_version": "0.1",
		"method": "approval.respond", "params": params,
	}
	if !s.Pool.SendTo(r.Context(), a.InstanceID, note) {
		writeJSON(w, 502, map[string]string{"error": "DELIVERY_FAILED"})
		return
	}
	writeJSON(w, 200, map[string]string{"approval_id": a.ApprovalID, "decision": a.Decision.String})
}

var _ = detect.MethodPTY
