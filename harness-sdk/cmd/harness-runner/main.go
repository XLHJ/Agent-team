package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"agent-control-plane/harness-sdk/detect"
	"agent-control-plane/harness-sdk/lifecycle"
	"agent-control-plane/harness-sdk/rpc"
	"agent-control-plane/harness-sdk/takeover"
	"agent-control-plane/harness-sdk/translate"
	"agent-control-plane/harness-sdk/validate"
	"agent-control-plane/harness-sdk/wire"
)

type dispatchParams struct {
	TaskID       string `json:"task_id"`
	Prompt       string `json:"prompt"`
	InputContext string `json:"input_context"`
	ReportPrompt string `json:"report_prompt"`
	TimeoutMS    int    `json:"timeout_ms"`
}

type decision struct {
	Decision string          `json:"decision"`
	Modified json.RawMessage `json:"modified_params"`
}

type runner struct {
	sig   detect.Signature
	path  string
	id    string
	lc    lifecycleState
	mu    sync.Mutex
	ch    takeover.Channel
	pend  map[string]chan decision
	queue []dispatchParams
	busy  bool
	ctrl  chan struct{}
	start chan struct{}
}

// lifecycleState narrows the surface used from lifecycle.Manager.
type lifecycleState interface {
	Connect(ctx context.Context) error
	Retry(ctx context.Context) error
	Drain()
	Close()
	Client() rpcClient
}

type rpcClient interface {
	Notify(ctx context.Context, method string, params any) error
}

func main() {
	sigName := flag.String("signature", "", "signature name")
	path := flag.String("path", "", "resolved CLI path")
	methods := flag.String("methods", "", "comma-ordered methods")
	schemas := flag.String("schemas", "../../../protocol/schemas", "schema dir")
	flag.Parse()

	hubURL := os.Getenv("HUB_WS_URL")
	token := os.Getenv("BOOTSTRAP_TOKEN")
	name := os.Getenv("ACP_AGENT_NAME")
	if name == "" {
		name = *sigName
	}
	if hubURL == "" || *sigName == "" || *path == "" {
		fmt.Fprintln(os.Stderr, "harness-runner: HUB_WS_URL, --signature and --path are required")
		os.Exit(2)
	}
	sig := detect.Signature{Name: *sigName, Methods: splitList(*methods)}
	method, ok := takeover.Choose(sig)
	if !ok {
		fmt.Fprintln(os.Stderr, "harness-runner: no takeover method")
		os.Exit(2)
	}
	val, err := validate.Load(*schemas)
	if err != nil {
		fmt.Fprintln(os.Stderr, "harness-runner:", err)
		os.Exit(2)
	}

	instanceID := idendity(name)
	caps := []string{wire.CapObserve}
	if method != detect.MethodPTY {
		caps = append(caps, wire.CapTaskDispatch, wire.CapReportGenerate)
		if mode := approvalMode(*sigName); mode != "" {
			caps = append(caps, wire.CapApproval)
		}
	}

	r := &runner{sig: sig, path: *path, id: instanceID, pend: map[string]chan decision{}, ctrl: make(chan struct{})}
	var lm *lifecycle.Manager
	lm = lifecycle.New(lifecycle.Config{
		HubURL:       hubURL,
		HTTPBase:     strings.TrimSuffix(strings.Replace(hubURL, "ws://", "http://", 1), "/ws"),
		Token:        token,
		InstanceID:   instanceID,
		AgentType:    "cli:" + name,
		AgentVersion: "0",
		Capabilities: caps,
		Metadata:     map[string]any{"approval_mode": approvalMode(*sigName), "method": method},
	}, lifecycle.Hooks{
		OnState:      func(s lifecycle.State) { reportState(lm, instanceID, s) },
		OnDisconnect: func(reason string) { fmt.Fprintf(os.Stdout, "EVENT disconnected %s\n", reason) },
		OnTaskLost: func(taskID string) {
			if cl := lm.Client(); cl != nil {
				_ = cl.Notify(context.Background(), wire.MTaskFailed, map[string]any{
					"task_id": taskID, "error_code": "TASK_LOST_ON_RECONNECT",
					"error_message": "harness restarted during the task",
				})
			}
		},
	}, val)
	r.lc = realLC{m: lm}

	if err := lm.Connect(context.Background()); err != nil {
		fmt.Fprintf(os.Stdout, "EVENT connect_failed %v\n", err)
	}
	go r.pushLoop()
	go r.controlLoop()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	if r.ch != nil {
		_ = r.ch.Interrupt()
	}
	lm.Close()
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func approvalMode(sig string) string {
	switch sig {
	case "claude-code", "codex":
		return wire.ApprovalToolHook
	case "hermes":
		return wire.ApprovalNativeForward
	}
	return ""
}

func (r *runner) controlLoop() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		switch strings.TrimSpace(sc.Text()) {
		case "retry":
			if err := r.lc.Retry(context.Background()); err != nil {
				fmt.Fprintf(os.Stdout, "EVENT retry_failed %v\n", err)
			}
		case "drain":
			r.lc.Drain()
		case "quit":
			close(r.ctrl)
			return
		}
	}
}

func (r *runner) pushLoop() {
	for {
		cl := r.lc.Client()
		if cl == nil {
			select {
			case <-time.After(100 * time.Millisecond):
				continue
			case <-r.ctrl:
				return
			}
		}
		rc, ok := cl.(*rpc.Client)
		if !ok {
			return
		}
		select {
		case <-r.ctrl:
			return
		case <-rc.Done():
		case f := <-rc.Push():
			r.onPush(f)
		}
	}
}

func (r *runner) onPush(f wire.Frame) {
	switch f.Method {
	case wire.MTaskDispatch:
		var p dispatchParams
		if json.Unmarshal(f.Params, &p) != nil {
			return
		}
		r.mu.Lock()
		if r.busy {
			r.queue = append(r.queue, p)
			r.mu.Unlock()
			return
		}
		r.busy = true
		r.mu.Unlock()
		go r.runTask(p)
	case wire.MTaskCancel, wire.MSessionInterrupt:
		r.mu.Lock()
		ch := r.ch
		r.mu.Unlock()
		if ch != nil {
			if f.Method == wire.MTaskCancel {
				_ = ch.Cancel()
			} else {
				_ = ch.Interrupt()
			}
		}
	case wire.MApprovalRespond:
		var p struct {
			ApprovalID string          `json:"approval_id"`
			Decision   string          `json:"decision"`
			Modified   json.RawMessage `json:"modified_params"`
		}
		if json.Unmarshal(f.Params, &p) != nil {
			return
		}
		r.mu.Lock()
		ch := r.pend[p.ApprovalID]
		r.mu.Unlock()
		if ch != nil {
			ch <- decision{Decision: p.Decision, Modified: p.Modified}
		}
	}
}

func (r *runner) runTask(p dispatchParams) {
	ctx := context.Background()
	cl := r.lc.Client()
	if cl == nil {
		r.finish()
		return
	}
	prompt := p.Prompt
	if p.InputContext != "" {
		prompt = p.InputContext
	}
	method, _ := takeover.Choose(r.sig)
	ch, err := takeover.Open(r.sig, r.path, method, translate.For(r.sig.Name))
	if err != nil {
		_ = cl.Notify(ctx, wire.MTaskFailed, map[string]any{"task_id": p.TaskID, "error_code": "CHANNEL_OPEN_FAILED", "error_message": err.Error()})
		r.finish()
		return
	}
	if err := ch.Start(); err != nil {
		_ = cl.Notify(ctx, wire.MTaskFailed, map[string]any{"task_id": p.TaskID, "error_code": "SPAWN_FAILED", "error_message": err.Error()})
		r.finish()
		return
	}
	r.mu.Lock()
	r.ch = ch
	r.mu.Unlock()

	if err := cl.Notify(ctx, wire.MTaskProgress, map[string]any{"task_id": p.TaskID, "progress": 5, "step": "dispatched"}); err != nil {
	} else {
	}
	if err := ch.SendTask(prompt); err != nil {
		_ = ch.Interrupt()
		_ = cl.Notify(ctx, wire.MTaskFailed, map[string]any{"task_id": p.TaskID, "error_code": "SEND_FAILED", "error_message": err.Error()})
		r.finish()
		return
	}

	progress := 10
	var out string
	evN := 0
	for ev := range ch.Events() {
		evN++
		switch ev.Kind {
		case takeover.EvProgress:
			progress += 10
			if progress > 90 {
				progress = 90
			}
			_ = cl.Notify(ctx, wire.MTaskProgress, map[string]any{"task_id": p.TaskID, "progress": progress, "step": truncate(ev.Step)})
		case takeover.EvLog:
			_ = cl.Notify(ctx, wire.MLogEvent, map[string]any{"instance_id": r.id, "task_id": p.TaskID, "level": "info", "message": truncate(ev.Step)})
		case takeover.EvApproval:
			r.askApproval(ctx, cl, ch, p.TaskID, ev)
		case takeover.EvCompleted:
			out = ev.RawOutput
			if out == "" {
				out = ch.PartialText()
			}
			// Per-task channel: the CLI session ends with the task. Stop
			// reading and tear the child down instead of waiting for EOF.
			_ = ch.Interrupt()
			goto drained
		case takeover.EvFailed:
			_ = cl.Notify(ctx, wire.MTaskFailed, map[string]any{"task_id": p.TaskID, "error_code": ev.ErrorCode, "error_message": ev.ErrorMessage})
			_ = ch.Interrupt()
			goto drained
		}
	}
drained:
	if out == "" {
		out = ch.PartialText()
	}
	if err := ch.ExitErr(); err != nil && out == "" {
		_ = cl.Notify(ctx, wire.MTaskFailed, map[string]any{"task_id": p.TaskID, "error_code": "AGENT_EXIT", "error_message": err.Error()})
		r.finish()
		return
	}
	report := out
	if p.ReportPrompt != "" {
		if r2 := r.followupReport(p, out); r2 != "" {
			report = r2
		}
	}
	if err := cl.Notify(ctx, wire.MTaskCompleted, map[string]any{"task_id": p.TaskID, "raw_output": out, "report": report}); err != nil {
	} else {
	}
	r.finish()
}

func (r *runner) followupReport(p dispatchParams, out string) string {
	method, _ := takeover.Choose(r.sig)
	ch, err := takeover.Open(r.sig, r.path, method, translate.For(r.sig.Name))
	if err != nil || ch.Start() != nil {
		return ""
	}
	defer ch.Close()
	_ = ch.SendTask(p.ReportPrompt + "\n\n任务输出：\n" + out)
	var b string
	for ev := range ch.Events() {
		if ev.Kind == takeover.EvCompleted && ev.RawOutput != "" {
			b = ev.RawOutput
		}
	}
	return b
}

func (r *runner) askApproval(ctx context.Context, cl rpcClient, ch takeover.Channel, taskID string, ev takeover.Event) {
	id := ev.ApprovalID
	if id == "" {
		id = fmt.Sprintf("ap-%d", time.Now().UnixNano())
	}
	wait := make(chan decision, 1)
	r.mu.Lock()
	r.pend[id] = wait
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pend, id)
		r.mu.Unlock()
	}()
	_ = cl.Notify(ctx, wire.MApprovalRequested, map[string]any{
		"approval_id": id, "task_id": taskID, "instance_id": r.id,
		"operation_type":  orDefault(ev.OperationType, "tool"),
		"risk_level":      orDefault(ev.RiskLevel, "medium"),
		"context_summary": orDefault(ev.ContextSummary, "approval requested"),
	})
	select {
	case d := <-wait:
		_ = ch.AnswerApproval(id, d.Decision, d.Modified)
	case <-time.After(10 * time.Minute):
		_ = ch.AnswerApproval(id, "deny", nil)
	}
}

func (r *runner) finish() {
	r.mu.Lock()
	if r.ch != nil {
		_ = r.ch.Close()
	}
	r.ch = nil
	r.busy = false
	var next *dispatchParams
	if len(r.queue) > 0 {
		p := r.queue[0]
		r.queue = r.queue[1:]
		next = &p
		r.busy = true
	}
	r.mu.Unlock()
	if next != nil {
		go r.runTask(*next)
	}
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// idendity keeps a stable instance_id across runner restarts on one machine:
// cli:<name>:<hash(name)> so the Hub rebinds the record and recovery lists
// previous-life tasks.
func idendity(name string) string {
	h := fnv32(name)
	return fmt.Sprintf("cli:%s:%s", name, hex32(h))
}

func fnv32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func hex32(v uint32) string {
	const digits = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = digits[v&0xf]
		v >>= 4
	}
	return string(b[:])
}
