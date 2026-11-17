//go:build !windows

package takeover

import (
	"os/exec"
	"syscall"
)

type jobHandle struct{}

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func assign(*exec.Cmd, *jobHandle) {}

func terminate(p *Proc) error {
	if p.cmd.Process == nil {
		return nil
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	return p.cmd.Process.Kill()
}

func closeJob(*jobHandle) {}
