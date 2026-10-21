//go:build unix

package dispatch

import (
	"os/exec"
	"syscall"
)

func ownGroup(cmd *exec.Cmd) error {
	// A new session and a group led by the worker: setpgid cannot cross
	// sessions, so no descendant can join a group of the Keeper's terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
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
