// fakeagent is a stand-in CLI for self-tests: --version plus a line-based
// structured-stdio task loop.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	ver := flag.Bool("version", false, "print version")
	flag.Parse()
	if *ver {
		fmt.Println("fakeagent 0.9.0")
		return
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	out := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		line := sc.Text()
		var p struct {
			Type   string `json:"type"`
			Prompt string `json:"prompt"`
		}
		if json.Unmarshal([]byte(line), &p) != nil || p.Type != "user" {
			continue
		}
		_ = out.Encode(map[string]string{"kind": "progress", "step": "working"})
		if rest, ok := slowMs(p.Prompt); ok {
			time.Sleep(time.Duration(rest) * time.Millisecond)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
		if strings.Contains(p.Prompt, "上一阶段") || strings.Contains(p.Prompt, "任务输出") {
			_ = out.Encode(map[string]string{"kind": "completed", "raw_output": "报告：完成 A/B，风险：无"})
			continue
		}
		_ = out.Encode(map[string]string{"kind": "completed", "raw_output": "完成：" + strings.TrimSpace(p.Prompt)})
	}
}

// slowMs parses an optional SLOW=<ms> prefix used by self-tests.
func slowMs(prompt string) (int, bool) {
	if !strings.HasPrefix(prompt, "SLOW=") {
		return 0, false
	}
	n := 0
	for _, c := range prompt[5:] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n, n > 0
}
