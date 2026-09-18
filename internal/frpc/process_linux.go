//go:build linux

package frpc

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func prepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func processAlive(pid int, expectedExecutable string) bool {
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		return false
	}
	if strings.TrimSpace(expectedExecutable) == "" {
		return true
	}

	actual, err := filepath.EvalSymlinks(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return false
	}
	expected, err := filepath.Abs(expectedExecutable)
	if err != nil {
		expected = expectedExecutable
	}
	actual = strings.TrimSuffix(actual, " (deleted)")
	actual, _ = filepath.Abs(actual)
	return filepath.Clean(actual) == filepath.Clean(expected)
}
