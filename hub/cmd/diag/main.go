// diag reproduces the S3 loop with full child output for debugging.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type instance struct {
	InstanceID string `json:"instance_id"`
	ConnStatus string `json:"conn_status"`
}

type taskRow struct {
	TaskID    string `json:"task_id"`
	State     string `json:"state"`
	RawOutput string `json:"raw_output"`
	Report    string `json:"report"`
	ErrorCode string `json:"error_code"`
}

func main() {
	base := "http://127.0.0.1:18300"
	root := "/workspace/agent-control-plane"

	hub := exec.Command(root+"/hub/hub",
		"-addr", "127.0.0.1:18300",
		"-schemas", root+"/protocol/schemas",
		"-signatures", root+"/harness-sdk/detect/signatures",
		"-state-dir", "", "-token", "t")
	hub.Stdout = os.Stdout
	hub.Stderr = os.Stdout
	if err := hub.Start(); err != nil {
		panic(err)
	}
	defer hub.Process.Kill()

	healthy := false
	for i := 0; i < 100; i++ {
		resp, err := http.Get(base + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				healthy = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !healthy {
		fmt.Println("hub never healthy")
		return
	}

	runner := exec.Command(root+"/harness-sdk/harness-runner",
		"-signature", "fakeagent",
		"-path", root+"/harness-sdk/fakeagent",
		"-methods", "stdio",
		"-schemas", root+"/protocol/schemas")
	runner.Env = append(os.Environ(),
		"HUB_WS_URL=ws://127.0.0.1:18300/ws",
		"BOOTSTRAP_TOKEN=t",
		"ACP_AGENT_NAME=fakeagent")
	runner.Stdout = os.Stdout
	runner.Stderr = os.Stdout
	if err := runner.Start(); err != nil {
		panic(err)
	}
	defer runner.Process.Kill()

	get := func(path string, v any) bool {
		req, _ := http.NewRequest("GET", base+path, nil)
		req.Header.Set("X-Hub-Token", "t")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return json.Unmarshal(b, v) == nil
	}
	var id string
	for i := 0; i < 100; i++ {
		var list []instance
		if get("/registry", &list) {
			for _, in := range list {
				if strings.HasPrefix(in.InstanceID, "cli:fakeagent:") && in.ConnStatus == "READY" {
					id = in.InstanceID
				}
			}
		}
		if id != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("instance:", id)
	if id == "" {
		return
	}
	body := fmt.Sprintf(`{"instance_id":"%s","task_id":"t-diag","prompt":"hello","timeout_ms":10000}`, id)
	req, _ := http.NewRequest("POST", base+"/selftest/tasks/dispatch", strings.NewReader(body))
	req.Header.Set("X-Hub-Token", "t")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("dispatch:", resp.StatusCode, string(b))

	var t taskRow
	for i := 0; i < 60; i++ {
		if get("/tasks/t-diag", &t) && (t.State == "completed" || t.State == "failed") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("final task: %+v\n", t)
	time.Sleep(500 * time.Millisecond)
}
