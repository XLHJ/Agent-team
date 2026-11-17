package main

import (
	"context"
	"fmt"
	"os"

	"agent-control-plane/harness-sdk/lifecycle"
	"agent-control-plane/harness-sdk/rpc"
	"agent-control-plane/harness-sdk/wire"
)

// realLC wraps lifecycle.Manager so main can stay testable.
type realLC struct{ m *lifecycle.Manager }

func (r realLC) Connect(ctx context.Context) error { return r.m.Connect(ctx) }
func (r realLC) Retry(ctx context.Context) error   { return r.m.Retry(ctx) }
func (r realLC) Drain()                            { r.m.Drain() }
func (r realLC) Close()                            { r.m.Close() }
func (r realLC) Client() rpcClient {
	if c := r.m.Client(); c != nil {
		return c
	}
	return nil
}

func (r realLC) State() string { return string(r.m.State()) }

// reportState prints and pushes agent.status on every state transition.
func reportState(lc *lifecycle.Manager, instanceID string, s lifecycle.State) {
	fmt.Fprintf(os.Stdout, "STATE %s\n", s)
	if lc == nil {
		return
	}
	cl := lc.Client()
	if cl == nil || instanceID == "" {
		return
	}
	_ = cl.Notify(context.Background(), wire.MAgentStatus, map[string]any{
		"instance_id": instanceID, "status": "idle",
	})
}

var _ = rpc.ErrClosed
