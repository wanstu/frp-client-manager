//go:build !windows

package frpc

import (
	"os"
	"os/exec"
)

func prepareCommand(*exec.Cmd) {}

func processAlive(pid int, _ string) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(os.Signal(nil)) == nil
}
