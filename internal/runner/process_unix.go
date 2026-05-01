//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcessTree(cmd *exec.Cmd) bool {
	if cmd.Process == nil {
		return true
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return false
	}
	termErr := syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(processTerminationGrace)
	killErr := syscall.Kill(-pgid, syscall.SIGKILL)
	return termErr == nil || killErr == nil
}
