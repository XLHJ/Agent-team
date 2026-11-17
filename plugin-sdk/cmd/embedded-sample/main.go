// embedded-sample registers an echo plugin via the embedded form and answers
// one task per dispatch. Phase-2 skeleton of a host-side adapter.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"agent-control-plane/harness-sdk/validate"

	"agent-control-plane/plugin-sdk/embedded"
	"agent-control-plane/plugin-sdk/shared"
)

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
		AgentType: "sample-embedded",
	}
	var p *shared.Plugin
	p, err = embedded.RegisterEmbeddedPlugin(context.Background(), cfg, shared.Hooks{
		OnTask: func(t shared.Task) {
			_ = p.Progress(t.TaskID, 50, "echoing")
			time.Sleep(50 * time.Millisecond)
			_ = p.Completed(t.TaskID, "echo: "+t.Prompt, "report: echo done")
		},
	}, "host exposes an in-process extension point", val)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("embedded plugin registered as", cfg.InstanceID)
	shared.Wait()
	p.Close()
}
