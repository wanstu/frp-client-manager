//go:build darwin

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

	output, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return false
	}
	actual := strings.TrimSpace(string(output))
	if actual == "" {
		return false
	}
	expected, err := filepath.Abs(expectedExecutable)
	if err != nil {
		expected = expectedExecutable
	}
	if filepath.IsAbs(actual) {
		return filepath.Clean(actual) == filepath.Clean(expected)
	}
	return filepath.Base(actual) == filepath.Base(expected)
}
