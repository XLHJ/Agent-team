// Package panel implements the Phase-3 panel channel: a token-guarded
// loopback WebSocket at /panel plus the panel.* JSON-RPC methods.
package panel

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"agent-control-plane/hub/internal/approvalsv2"
	"agent-control-plane/hub/internal/files"
	"agent-control-plane/hub/internal/intake"
	"agent-control-plane/hub/internal/registry"
	"agent-control-plane/hub/internal/store"
	"agent-control-plane/hub/internal/takeover"
	"agent-control-plane/hub/internal/tasks"
	"agent-control-plane/hub/internal/workflows"
	"agent-control-plane/hub/internal/ws"
)

// Deps wires the panel onto hub state.
type Deps struct {
	Reg    *registry.Registry
	Tasks  *tasks.Store
	Appr   *approvalsv2.Engine
	Pool   *ws.Pool
	Intake *intake.Manager
	Wf     *workflows.Chain
	Files  *files.Service
	Store  *store.DB
	Bus    *Bus
	Take   *takeover.Manager
	Log    *log.Logger
}

// Bus fans lifecycle events out to panel sessions (drop on slow readers).
type Bus struct {
	mu   sync.Mutex
	subs []chan map[string]any
}

// NewBus creates an event bus.
func NewBus() *Bus { return &Bus{} }

// Publish delivers evt to every subscriber without blocking.
func (b *Bus) Publish(evt map[string]any) {
	b.mu.Lock()
	subs := append([]chan map[string]any{}, b.subs...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

// SerEvent builds the panel.event frame params.
func SerEvent(typ string, payload any) map[string]any {
	return map[string]any{
		"event_type": typ,
		"ts":         time.Now().UTC(),
		"payload":    payload,
	}
}

// Handler serves GET /panel.
type Handler struct {
	d     Deps
	token string
	bus   *Bus
	mu    sync.Mutex
	busy  bool
}

// New builds the handler.
func New(d Deps, token string, bus *Bus) *Handler {
	return &Handler{d: d, token: token, bus: bus}
}

// allowedOrigin is the doc 4.1 whitelist: file:// pages send "null",
// dev uses the Vite origin, non-browser clients send nothing.
func allowedOrigin(o string) bool {
	return o == "" || o == "null" || o == "http://localhost:5173"
}

// Handle upgrades one panel session (single-panel constraint, doc 4.1).
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	if h.token != "" && r.URL.Query().Get("token") != h.token {
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
		return
	}
	if !allowedOrigin(r.Header.Get("Origin")) {
		http.Error(w, "BAD_ORIGIN", http.StatusForbidden)
		return
	}
	h.mu.Lock()
	if h.busy {
		h.mu.Unlock()
		http.Error(w, "PANEL_BUSY", http.StatusConflict)
		return
	}
	h.busy = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.busy = false
		h.mu.Unlock()
	}()
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	h.session(r.Context(), c)
}

// rpcError is a JSON-RPC error with an optional business code.
type rpcError struct {
	code int
	bus  string
	msg  string
}

func bizErr(bus, msg string) *rpcError { return &rpcError{code: -32001, bus: bus, msg: msg} }

// session runs request handling plus the event pump.
func (h *Handler) session(ctx context.Context, c *websocket.Conn) {
	ch := make(chan map[string]any, 256)
	h.bus.mu.Lock()
	h.bus.subs = append(h.bus.subs, ch)
	h.bus.mu.Unlock()
	defer func() {
		h.bus.mu.Lock()
		for i, s := range h.bus.subs {
			if s == ch {
				h.bus.subs = append(h.bus.subs[:i], h.bus.subs[i+1:]...)
				break
			}
		}
		h.bus.mu.Unlock()
	}()
	flt := &filter{}
	go h.pump(ctx, c, ch, flt)
	for {
		_, raw, err := c.Read(ctx)
		if err != nil {
			return
		}
		req := struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}{}
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		res, rerr := h.dispatch(ctx, flt, req.Method, req.Params)
		h.reply(ctx, c, req.ID, res, rerr)
	}
}

func (h *Handler) pump(ctx context.Context, c *websocket.Conn, ch chan map[string]any, flt *filter) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			if !flt.match(evt["event_type"].(string)) {
				continue
			}
			frame := map[string]any{"jsonrpc": "2.0", "method": "panel.event", "params": evt}
			b, err := json.Marshal(frame)
			if err != nil {
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, b); err != nil {
				return
			}
		}
	}
}

func (h *Handler) reply(ctx context.Context, c *websocket.Conn, id int64, res any, rerr *rpcError) {
	out := map[string]any{"jsonrpc": "2.0", "id": id}
	if rerr != nil {
		errObj := map[string]any{"code": rerr.code, "message": rerr.msg}
		if rerr.bus != "" {
			errObj["data"] = map[string]any{"code": rerr.bus}
		}
		out["error"] = errObj
	} else {
		if res == nil {
			res = map[string]any{}
		}
		out["result"] = res
	}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	_ = c.Write(ctx, websocket.MessageText, b)
}

// filter matches event types with "task.*" style wildcards; empty = all.
type filter struct{ pats []string }

func (f *filter) match(typ string) bool {
	if f == nil || len(f.pats) == 0 {
		return true
	}
	for _, p := range f.pats {
		if p == typ || (len(p) > 1 && p[len(p)-1] == '*' && len(typ) >= len(p)-1 && typ[:len(p)-1] == p[:len(p)-1]) {
			return true
		}
	}
	return false
}

func (h *Handler) dispatch(ctx context.Context, flt *filter, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "panel.subscribe":
		var p struct {
			Types []string `json:"types"`
		}
		_ = json.Unmarshal(params, &p)
		flt.pats = p.Types
		return map[string]any{}, nil
	case "panel.unsubscribe":
		flt.pats = nil
		return map[string]any{}, nil
	case "panel.agents.list":
		return map[string]any{"instances": h.d.Reg.Snapshot()}, nil
	case "panel.tasks.list":
		return h.tasksList(params)
	case "panel.tasks.get":
		return h.taskGet(params)
	case "panel.tasks.create":
		return h.taskCreate(ctx, params)
	case "panel.tasks.cancel":
		return h.taskCancel(ctx, params)
	case "panel.approvals.list":
		return h.approvalsList(params)
	case "panel.approvals.respond":
		return h.approvalsRespond(ctx, params)
	case "panel.intake.candidates":
		return map[string]any{"candidates": h.d.Intake.List()}, nil
	case "panel.workflows.list":
		return h.workflowsList(params)
	case "panel.workflows.get":
		return h.workflowGet(params)
	case "panel.workflows.save":
		return h.workflowSave(params)
	case "panel.workflows.start":
		return h.workflowStart(ctx, params)
	case "panel.workflows.runs":
		return h.workflowRuns(params)
	case "panel.workflows.run":
		return h.workflowRunGet(params)
	case "panel.workflows.cancel":
		return h.workflowCancel(params)
	case "panel.workflows.delete":
		return h.workflowDelete(params)
	case "panel.workflows.export":
		return h.workflowExport(params)
	case "panel.workflows.import":
		return h.workflowImport(params)
	case "panel.intake.confirm":
		return h.intakeConfirm(params)
	case "panel.intake.ignore":
		return h.intakeIgnore(params)
	case "panel.files.list":
		return h.filesList(params)
	case "panel.files.read":
		return h.filesRead(params)
	case "panel.usage.summary":
		return h.usageSummary(params)
	case "panel.usage.logs":
		return h.usageLogs(params)
	case "panel.kitesurf.get":
		return h.kitesurfGet(params)
	case "panel.kitesurf.set":
		return h.kitesurfSet(params)
	}
	return nil, &rpcError{code: -32601, msg: "method not found"}
}
