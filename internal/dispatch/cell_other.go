//go:build !unix

package dispatch

import "os/exec"

// Process groups and signals need a unix host (Q-T023-17).
func ownGroup(*exec.Cmd) error { return failf(ExitEnv, "waggle dispatch needs a unix host") }

func signalGroup(*exec.Cmd, bool) {}
