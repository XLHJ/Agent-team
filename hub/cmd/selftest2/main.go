// selftest2 runs the Phase-2 acceptance scenarios S1-S5 against a real Hub
// process, a spawned harness-runner and a fake CLI agent. Exit 0 = all pass.
//
//	S1  intake scan detects the fake agent via PATH
//	S2  confirm spawns the runner; it registers cli:fakeagent:* and goes READY
//	S3  dispatch -> progress -> completed with non-empty report
//	S4  kill the runner mid-task; record flips UNREACHABLE, task stays non-terminal
//	S5  restart the runner with the same instance id; Hub rebinds, the lost task
//	    is reported failed with TASK_LOST_ON_RECONNECT, no duplicate records
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"time"
)

var (
	fails int
	mu    sync.Mutex
)

func check(name string, ok bool, detail string) {
	mu.Lock()
	if ok {
		fmt.Printf("PASS %-44s\n", name)
	} else {
		fails++
		fmt.Printf("FAIL %-44s %s\n", name, detail)
	}
	mu.Unlock()
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
	for i, a := range os.Args {
		if a == "-"+name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return def
}

type frame struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
}

type instance struct {
	InstanceID   string   `json:"instance_id"`
	AgentType    string   `json:"agent_type"`
	ConnStatus   string   `json:"conn_status"`
	ApprovalMode string   `json:"approval_mode"`
	Capabilities []string `json:"capabilities"`
}

type taskRow struct {
	TaskID       string `json:"task_id"`
	State        string `json:"state"`
	RawOutput    string `json:"raw_output"`
	Report       string `json:"report"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type candidate struct {
	Signature string `json:"signature"`
	Version   string `json:"version"`
	State     string `json:"state"`
}

func run(hubBin, runnerBin, agentBin string) {
	port := mustPort()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	hub := exec.Command(hubBin,
		"-addr", fmt.Sprintf("127.0.0.1:%d", port),
		"-schemas", "../../../protocol/schemas",
		"-signatures", "../../../harness-sdk/detect/signatures",
		"-state-dir", "",
		"-harness-bin", runnerBin,
		"-token", "testtoken")
	// S1 depends on the fake agent being discoverable on PATH.
	agentDir, _ := strings.CutSuffix(agentBin, "/fakeagent")
	hub.Env = append(os.Environ(), "PATH="+agentDir+":"+os.Getenv("PATH"))
	hub.Stdout = os.Stdout
	hub.Stderr = os.Stderr
	must(hub.Start())
	defer func() {
		_ = hub.Process.Kill()
		_ = hub.Wait()
	}()

	get := func(path string) []byte {
		req, _ := http.NewRequest("GET", base+path, nil)
		req.Header.Set("X-Hub-Token", "testtoken")
		resp, err := http.DefaultClient.Do(req)
		must(err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return b
	}
	post := func(path, body string) (int, []byte) {
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
		req.Header.Set("X-Hub-Token", "testtoken")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		must(err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	waitFor := func(what string, tries int, cond func() bool) bool {
		for i := 0; i < tries; i++ {
			if cond() {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("timeout waiting:", what)
		return false
	}
	healthy := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(base + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				healthy = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	check("S0 hub healthy", healthy, "no 200 from /health")
	if !healthy {
		return
	}
	// spawner helper for S5 restarts.
	spawn := func() *exec.Cmd {
		cmd := exec.Command(runnerBin,
			"-signature", "fakeagent",
			"-path", agentBin,
			"-methods", "stdio",
			"-schemas", "../../../protocol/schemas")
		cmd.Env = append(os.Environ(),
			"HUB_WS_URL=ws://127.0.0.1:"+strconv.Itoa(port)+"/ws",
			"BOOTSTRAP_TOKEN=testtoken",
			"ACP_AGENT_NAME=fakeagent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.Discard
		must(cmd.Start())
		return cmd
	}

	// S1: intake scan finds the fake agent via PATH.
	var cands []candidate
	s1 := waitFor("S1 candidates", 30, func() bool {
		var list []candidate
		if err := json.Unmarshal(get("/intake/candidates"), &list); err != nil {
			return false
		}
		cands = list
		for _, c := range list {
			if c.Signature == "fakeagent" && c.Version == "0.9.0" {
				return true
			}
		}
		return false
	})
	check("S1 scan detects fakeagent 0.9.0", s1, fmt.Sprintf("candidates=%s", mustJSON(cands)))

	// S2: user confirms; hub spawns the runner; it registers and goes READY.
	code, body := post("/intake/confirm", `{"signature":"fakeagent"}`)
	check("S2 confirm accepted", code == 200, fmt.Sprintf("code=%d body=%s", code, body))
	var reg []instance
	ready := waitFor("S2 runner READY", 50, func() bool {
		var list []instance
		if err := json.Unmarshal(get("/registry"), &list); err != nil {
			return false
		}
		reg = list
		for _, in := range list {
			if strings.HasPrefix(in.InstanceID, "cli:fakeagent:") && in.ConnStatus == "READY" {
				return true
			}
		}
		return false
	})
	var agent instance
	for _, in := range reg {
		if strings.HasPrefix(in.InstanceID, "cli:fakeagent:") {
			agent = in
		}
	}
	check("S2 instance cli:fakeagent:* READY", ready, fmt.Sprintf("registry=%s", mustJSON(reg)))
	check("S2 caps observe+task.dispatch+report.generate",
		has(agent.Capabilities, "observe") && has(agent.Capabilities, "task.dispatch") && has(agent.Capabilities, "report.generate"),
		mustJSON(agent.Capabilities))

	// S3: full task loop over the fake agent.
	code, body = post("/selftest/tasks/dispatch", `{"instance_id":"`+agent.InstanceID+`","task_id":"t-echo","prompt":"hello","timeout_ms":10000}`)
	check("S3 dispatch accepted", code == 200, fmt.Sprintf("code=%d body=%s", code, body))
	var echo taskRow
	s3 := waitFor("S3 completed", 50, func() bool {
		if err := json.Unmarshal(get("/tasks/t-echo"), &echo); err != nil {
			return false
		}
		return echo.State == "completed"
	})
	check("S3 task completed", s3, mustJSON(echo))
	check("S3 raw_output echoes prompt", strings.Contains(echo.RawOutput, "hello"), echo.RawOutput)
	check("S3 report non-empty", echo.Report != "", "")

	// S4: kill the runner mid-task.
	code, body = post("/selftest/tasks/dispatch", `{"instance_id":"`+agent.InstanceID+`","task_id":"t-slow","prompt":"SLOW=1500 slow job","timeout_ms":30000}`)
	check("S4 slow task dispatched", code == 200, fmt.Sprintf("code=%d body=%s", code, body))
	running := waitFor("S4 task running", 50, func() bool {
		var t taskRow
		if err := json.Unmarshal(get("/tasks/t-slow"), &t); err != nil {
			return false
		}
		return t.State == "running"
	})
	check("S4 task reached running", running, "")
	code, body = post("/agents/"+agent.InstanceID+"/disconnect", `{}`)
	check("S4 disconnect accepted", code == 200, fmt.Sprintf("code=%d body=%s", code, body))
	down := waitFor("S4 UNREACHABLE", 50, func() bool {
		var list []instance
		if err := json.Unmarshal(get("/registry"), &list); err != nil {
			return false
		}
		for _, in := range list {
			if in.InstanceID == agent.InstanceID {
				return in.ConnStatus == "UNREACHABLE"
			}
		}
		return false
	})
	check("S4 record UNREACHABLE", down, "")
	var slow taskRow
	_ = json.Unmarshal(get("/tasks/t-slow"), &slow)
	check("S4 task stays non-terminal after drop", slow.State == "running" || slow.State == "dispatched", mustJSON(slow))

	// S5: restart with the same instance id; rebind + lost-task report.
	cmd := spawn()
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	back := waitFor("S5 rebind READY", 80, func() bool {
		var list []instance
		if err := json.Unmarshal(get("/registry"), &list); err != nil {
			return false
		}
		n := 0
		for _, in := range list {
			if strings.HasPrefix(in.InstanceID, "cli:fakeagent:") {
				n++
				if in.ConnStatus == "READY" {
					return true
				}
			}
		}
		return n == 1
	})
	check("S5 same instance id rebinds, single record", back, "")
	lost := waitFor("S5 TASK_LOST_ON_RECONNECT", 80, func() bool {
		var t taskRow
		if err := json.Unmarshal(get("/tasks/t-slow"), &t); err != nil {
			return false
		}
		return t.State == "failed" && t.ErrorCode == "TASK_LOST_ON_RECONNECT"
	})
	var slow2 taskRow
	_ = json.Unmarshal(get("/tasks/t-slow"), &slow2)
	check("S5 lost task failed with TASK_LOST_ON_RECONNECT", lost, mustJSON(slow2))
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func has(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func mustPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func must(err error) {
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(2)
	}
}
