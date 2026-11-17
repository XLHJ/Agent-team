// Hub entry point: WebSocket + JSON-RPC control plane with intake of CLI agents.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agent-control-plane/harness-sdk/detect"

	"agent-control-plane/hub/internal/approvalsv2"
	"agent-control-plane/hub/internal/files"
	"agent-control-plane/hub/internal/intake"
	"agent-control-plane/hub/internal/panel"
	"agent-control-plane/hub/internal/registry"
	"agent-control-plane/hub/internal/rpc"
	"agent-control-plane/hub/internal/server"
	"agent-control-plane/hub/internal/store"
	"agent-control-plane/hub/internal/takeover"
	"agent-control-plane/hub/internal/tasks"
	"agent-control-plane/hub/internal/workflows"
	"agent-control-plane/hub/internal/ws"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	schemas := flag.String("schemas", "../protocol/schemas", "protocol schema dir")
	sigs := flag.String("signatures", "../harness-sdk/detect/signatures", "CLI signature dir")
	stateDir := flag.String("state-dir", defaultStateDir(), "state dir (token file)")
	harnessBin := flag.String("harness-bin", "", "harness-runner binary (intake spawn)")
	workspace := flag.String("workspace", os.Getenv("ACP_WORKSPACE"), "workspace root for panel source preview (empty disables)")
	tokenEnv := flag.String("token", "", "fixed bootstrap token (default: random, written to state dir)")
	flag.Parse()

	lg := log.New(os.Stdout, "hub ", log.LstdFlags|log.Lmicroseconds)
	val, err := rpc.NewValidator(*schemas)
	if err != nil {
		lg.Fatalf("load schemas: %v", err)
	}
	token := *tokenEnv
	if token == "" {
		token = randomToken()
	}
	state := *stateDir
	if state != "" {
		if err := os.MkdirAll(state, 0o700); err != nil {
			lg.Fatalf("state dir: %v", err)
		}
		tokPath := filepath.Join(state, "hub.token")
		if err := os.WriteFile(tokPath, []byte(token), 0o600); err != nil {
			lg.Fatalf("write token: %v", err)
		}
		lg.Printf("bootstrap token written to %s", tokPath)
	}

	table, err := detect.Load(*sigs)
	if err != nil {
		lg.Fatalf("load signatures: %v", err)
	}

	reg := registry.New()
	tsk := tasks.New()
	appDB, err := store.Open(filepath.Join(state, "hub.db"))
	if err != nil {
		lg.Fatalf("open sqlite: %v", err)
	}
	defer appDB.Close()
	if err := appDB.Schema(); err != nil {
		lg.Fatalf("schema: %v", err)
	}
	app := approvalsv2.New(appDB, lg)
	pool := ws.NewPool()
	wfStore, wfErr := workflows.NewStore(appDB)
	if wfErr != nil {
		lg.Fatalf("workflow store: %v", wfErr)
	}
	wfEng := workflows.NewEngine(wfStore, nil, lg)
	wfChain := workflows.NewChain(wfStore, wfEng, lg)
	bus := panel.NewBus()
	rt := rpc.NewRouter(rpc.Deps{Reg: reg, Tasks: tsk, Appr: app, Wf: wfEng, Pool: pool, Val: val, Log: lg, Store: appDB, Bus: bus})
	srv := server.New(reg, tsk, app, pool, rt, lg, appDB)
	srv.Token = token
	srv.Sigs = table
	srv.Intake = intake.New()
	srv.Take = takeover.New(*harnessBin, "ws://"+*addr+"/ws", token, lg)
	var fileSvc *files.Service
	if *workspace != "" {
		fileSvc, err = files.New(*workspace)
		if err != nil {
			lg.Fatalf("workspace: %v", err)
		}
		lg.Printf("source preview enabled: workspace=%s", fileSvc.Root())
	}
	ph := panel.New(panel.Deps{
		Reg: reg, Tasks: tsk, Appr: app, Pool: pool,
		Intake: srv.Intake, Bus: bus, Take: srv.Take, Wf: wfChain, Log: lg,
		Files: fileSvc, Store: appDB,
	}, token, bus)
	srv.Panel = ph
	srv.Bus = bus
	app.Subscribe(func(ev approvalsv2.Event) {
		bus.Publish(panel.SerEvent(ev.Type, json.RawMessage(ev.Approval)))
	})
	// Phase-5: workflow stage binding and dispatch.
	wfEng.SetBinding(
		func(id string) bool {
			return reg.HasCapability(id, "report.generate")
		},
		func(tag string) (string, bool) {
			for _, in := range reg.Snapshot() {
				if !reg.HasCapability(in.InstanceID, "report.generate") {
					continue
				}
				if tag != "" && !hasTag(in.Capabilities, tag) {
					continue
				}
				return in.InstanceID, true
			}
			return "", false
		},
	)
	wfEng.SetDispatcher(func(ctx context.Context, instanceID, taskID, prompt, inputCtx, reportPrompt string, timeoutMS int) error {
		tsk.Dispatch(taskID, instanceID, prompt, timeoutMS)
		params := map[string]any{
			"task_id": taskID, "prompt": prompt, "timeout_ms": timeoutMS,
		}
		if reportPrompt != "" {
			params["report_prompt"] = reportPrompt
		}
		if inputCtx != "" {
			params["input_context"] = inputCtx
		}
		note := map[string]any{
			"jsonrpc": "2.0", "protocol_version": "0.1",
			"method": "task.dispatch", "params": params,
		}
		if !pool.SendTo(ctx, instanceID, note) {
			return fmt.Errorf("instance %s unreachable", instanceID)
		}
		return nil
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go srv.Sweep(ctx, 90*time.Second)

	// Phase-4: recover pending approvals and re-arm their timers (doc 6.2).
	if err := app.Recover(); err != nil {
		lg.Printf("approval recover: %v", err)
	}

	// Phase-5: interrupted runs fail fast on boot (doc 7.2).
	wfEng.Recover()
	// Phase-4: route timeout decisions back to the plugin (doc 2.4).
	app.SetAutoResolve(func(row *store.ApprovalRow) {
		note := map[string]any{
			"jsonrpc": "2.0", "protocol_version": "0.1",
			"method": "approval.respond",
			"params": map[string]any{
				"approval_id": row.ApprovalID, "decision": "deny",
				"reason": "timeout",
			},
		}
		if !pool.SendTo(context.Background(), row.InstanceID, note) {
			lg.Printf("timeout decision undeliverable approval=%s", row.ApprovalID)
		}
	})

	// Phase-4: exemptions are per-task; clear them on task terminal states.
	tsk.OnTerminal(func(taskID string) {
		if n := app.ClearExempts(taskID); n > 0 {
			lg.Printf("cleared %d exemption(s) for task=%s", n, taskID)
		}
	})

	// Usage: fold new detail rows into hourly/daily aggregates shortly after
	// boot and then every 5 minutes; raw logs expire after 31 days (doc 6.3).
	usageAggregate := func() {
		if n, err := appDB.UsageAggregate(); err != nil {
			lg.Printf("usage aggregate: %v", err)
		} else if n > 0 {
			lg.Printf("usage aggregate: %d row(s) folded", n)
		}
	}
	usageAggregate()
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				usageAggregate()
			}
		}
	}()
	go func() {
		app.Cleanup(30 * 24 * time.Hour)
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				app.Cleanup(30 * 24 * time.Hour)
				if n, err := wfStore.CleanupRuns(90 * 24 * time.Hour); err == nil && n > 0 {
					lg.Printf("workflow cleanup: %d run(s) removed", n)
				}
				if n, err := appDB.UsageCleanup(31 * 24 * time.Hour); err == nil && n > 0 {
					lg.Printf("usage cleanup: %d log row(s) removed", n)
				}
			}
		}
	}()

	// One scan at startup; the panel may trigger rescans (Phase-2 4.5).
	cands := srv.Intake.Scan(table)
	lg.Printf("intake scan: %d candidate(s)", len(cands))

	hs := &http.Server{Addr: *addr, Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
		srv.Take.CloseAll()
	}()
	lg.Printf("hub listening on %s (schemas=%s sigs=%s)", *addr, *schemas, *sigs)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		lg.Fatalf("serve: %v", err)
	}
	app.Close()
}

func defaultStateDir() string {
	if v := os.Getenv("ACP_STATE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agent-control-plane")
}

func randomToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// writePortFile publishes the port for the Electron shell (doc 4.1).
func writePortFile(stateDir, addr string) {
	if stateDir == "" {
		return
	}
	i := len(addr) - 1
	for i >= 0 && addr[i] != ':' {
		i--
	}
	if i < 0 {
		return
	}
	if err := os.WriteFile(filepath.Join(stateDir, "hub.port"),
		[]byte(addr[i+1:]+"\n"), 0o600); err != nil {
		log.Printf("write hub.port: %v", err)
	}
}

// hasTag reports whether a capability list contains a capability tag.
func hasTag(caps []string, tag string) bool {
	for _, c := range caps {
		if c == tag {
			return true
		}
	}
	return false
}
