//go:build windows

package takeover

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type jobHandle struct{ h windows.Handle }

func prepare(*exec.Cmd) {}

// assign puts the child into a kill-on-close Job Object so Hub crashes never
// leave orphan runners behind.
func assign(cmd *exec.Cmd, out *jobHandle) {
	if cmd.Process == nil {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph)
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(h)
		return
	}
	if err := windows.AssignProcessToJobObject(h, ph); err != nil {
		windows.CloseHandle(h)
		return
	}
	*out = jobHandle{h: h}
}

func terminate(p *Proc) error {
	if p.job.h != 0 {
		return windows.TerminateJobObject(p.job.h, 1)
	}
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

func closeJob(j *jobHandle) {
	if j.h != 0 {
		windows.CloseHandle(j.h)
		j.h = 0
	}
}
