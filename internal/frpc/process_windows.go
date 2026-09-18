//go:build windows

package frpc

import (
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func prepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
}

func processAlive(pid int, expectedExecutable string) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		return false
	}
	if strings.TrimSpace(expectedExecutable) == "" {
		return true
	}

	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return false
	}
	actual := windows.UTF16ToString(buf[:size])
	expected, err := filepath.Abs(expectedExecutable)
	if err != nil {
		expected = expectedExecutable
	}
	actual, _ = filepath.Abs(actual)
	return strings.EqualFold(filepath.Clean(actual), filepath.Clean(expected))
}
