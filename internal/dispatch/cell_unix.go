//go:build unix

package dispatch

import (
	"os/exec"
	"syscall"
)

func ownGroup(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

// signalGroup sends SIGKILL (hard) or SIGTERM to the worker's whole group.
func signalGroup(cmd *exec.Cmd, hard bool) {
	sig := syscall.SIGTERM
	if hard {
		sig = syscall.SIGKILL
	}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, sig)
	}
}
