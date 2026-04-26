//go:build windows

package runner

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}

func terminateProcessTree(cmd *exec.Cmd) bool {
	if cmd.Process != nil {
		// Native Windows execution is gated off in v0.1. Replace this with
		// taskkill /T /F or a job object before removing that platform gate.
		return cmd.Process.Kill() == nil
	}
	return true
}
