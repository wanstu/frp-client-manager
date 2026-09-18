//go:build !windows && !linux && !darwin

package frpc

import "os/exec"

func prepareCommand(*exec.Cmd) {}

func processAlive(int, string) bool { return false }
