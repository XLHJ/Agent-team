//go:build windows

package takeover

import (
	"os/exec"
	"strconv"
)

func configureGroup(*exec.Cmd) {}

// killTree terminates the whole child tree via taskkill /T.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}

func assignToJob(*exec.Cmd) {}
