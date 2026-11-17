// selftest4 runs the Phase-5 workflow scenarios W1-W6 against the Hub binary.
//
//	W1  save a two-stage workflow via panel.workflows.save
//	W2  start the run; stage 0 dispatches to the fake agent
//	W3  stage 0 completes with report; stage 1 input_context carries the handoff
//	W4  a stage completing without report fails the run with REPORT_MISSING
//	W5  cancel a running chain
//	W6  boot recovery: running runs marked failed with HUB_RESTART_INTERRUPTED
//	W7  export the chain as chain.v1 JSON via panel.workflows.export
//	W8  import it back; new id, draft status, definition preserved
//	W9  panel.files.list lists the workspace docs dir
//	W10 path traversal outside the workspace is rejected
//	W11 panel.files.read returns text with a Monaco language hint
//	W12 usage.report persisted + panel.usage.summary grouped aggregation
//	W13 invalid usage.report (missing calls) rejected
//	W14 panel.usage.logs returns the raw detail rows
//	W15 bridge /bridge/v1/agents lists dispatch-capable agents
//	W16 bridge async dispatch returns task_id immediately
//	W17 bridge async result becomes completed via status polling
//	W18 bridge sync dispatch waits and returns the completed result
//	W19 panel.kitesurf.set/get stores the browser WS endpoint hub-side
package main

import (
	"context"
	"fmt"
	"os"
)

var fails int

func check(name string, ok bool, detail string) {
	if ok {
		fmt.Printf("PASS %-44s\n", name)
	} else {
		fails++
		fmt.Printf("FAIL %-44s %s\n", name, detail)
	}
}

func main() {
	hubBin := arg("hub", "../../hub")
	runnerBin := arg("runner", "../../../harness-sdk/harness-runner")
	agentBin := arg("agent", "../../../harness-sdk/fakeagent")
	for _, b := range []string{hubBin, runnerBin, agentBin} {
		if _, err := os.Stat(b); err != nil {
			fmt.Println("missing binary:", b, "- build it first")
			os.Exit(2)
		}
	}
	run(hubBin, runnerBin, agentBin)
	fmt.Printf("TOTAL fails=%d\n", fails)
	if fails > 0 {
		os.Exit(1)
	}
}

func arg(name, def string) string {
	for i, a := range os.Args[1:] {
		if a == "-"+name && i+1 < len(os.Args) {
			return os.Args[i+2]
		}
	}
	return def
}

var _ = context.Background
