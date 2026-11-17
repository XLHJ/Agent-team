// sidecar-sample runs the plugin out of process and registers as sidecar form.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"agent-control-plane/harness-sdk/validate"

	"agent-control-plane/plugin-sdk/shared"
	"agent-control-plane/plugin-sdk/sidecar"
)

type adapter struct{}

func (adapter) ApprovalMode() string     { return "tool_hook" }
func (adapter) FormChoiceReason() string { return "host has no in-process extension point" }

func main() {
	schemas := flag.String("schemas", "../../../protocol/schemas", "schema dir")
	flag.Parse()
	val, err := validate.Load(*schemas)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cfg := shared.Config{
		HubURL:    os.Getenv("HUB_WS_URL"),
		HTTPBase:  os.Getenv("HUB_HTTP_BASE"),
		Token:     os.Getenv("BOOTSTRAP_TOKEN"),
		AgentType: "sample-sidecar",
	}
	var p *shared.Plugin
	p, err = sidecar.RunSidecarPlugin(context.Background(), cfg, adapter{}, shared.Hooks{
		OnTask: func(t shared.Task) {
			_ = p.Progress(t.TaskID, 50, "echoing")
			time.Sleep(50 * time.Millisecond)
			_ = p.Completed(t.TaskID, "echo: "+t.Prompt, "")
		},
	}, val)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("sidecar plugin registered")
	shared.Wait()
	p.Close()
}
