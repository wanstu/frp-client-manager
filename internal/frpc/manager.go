package frpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State struct {
	Running    bool     `json:"running"`
	PID        int      `json:"pid"`
	Detached   bool     `json:"detached"`
	Executable string   `json:"executable"`
	ConfigPath string   `json:"config_path"`
	StartedAt  string   `json:"started_at"`
	LastError  string   `json:"last_error"`
	LogTail    []string `json:"log_tail"`
}

type pidRecord struct {
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	ConfigPath string `json:"config_path"`
	StartedAt  string `json:"started_at"`
}

type Manager struct {
	mu        sync.Mutex
	dir       string
	pidPath   string
	logPath   string
	cmd       *exec.Cmd
	lastError string
}

func NewManager(dataDir string) *Manager {
	return &Manager{
		dir:     dataDir,
		pidPath: filepath.Join(dataDir, "frpc.pid.json"),
		logPath: filepath.Join(dataDir, "frpc.log"),
	}
}

func (m *Manager) Status() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) Start(executable, configPath string) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current := m.statusLocked(); current.Running {
		return current, nil
	}

	resolvedExecutable, err := resolveExecutable(executable)
	if err != nil {
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	absoluteConfig, err := filepath.Abs(strings.TrimSpace(configPath))
	if err != nil {
		err = fmt.Errorf("resolve frpc config path: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	if info, err := os.Stat(absoluteConfig); err != nil || info.IsDir() {
		if err == nil {
			err = errors.New("path is a directory")
		}
		err = fmt.Errorf("frpc config is not readable: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		err = fmt.Errorf("create application data directory: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}

	logFile, err := os.OpenFile(m.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		err = fmt.Errorf("open frpc log: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	_, _ = fmt.Fprintf(logFile, "\n[%s] starting frpc %s -c %s\n", time.Now().Format(time.RFC3339), resolvedExecutable, absoluteConfig)

	cmd := exec.Command(resolvedExecutable, "-c", absoluteConfig)
	prepareCommand(cmd)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		err = fmt.Errorf("start frpc: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	_ = logFile.Close()

	record := pidRecord{
		PID:        cmd.Process.Pid,
		Executable: resolvedExecutable,
		ConfigPath: absoluteConfig,
		StartedAt:  time.Now().Format(time.RFC3339),
	}
	if err := m.writeRecord(record); err != nil {
		_ = cmd.Process.Kill()
		err = fmt.Errorf("persist frpc process state: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	m.cmd = cmd
	m.lastError = ""
	go m.wait(cmd, record.PID)
	return m.statusLocked(), nil
}

func (m *Manager) Stop() (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.readRecord()
	if !ok {
		m.cmd = nil
		return m.statusLocked(), nil
	}
	if !processAlive(record.PID, record.Executable) {
		_ = os.Remove(m.pidPath)
		m.cmd = nil
		return m.statusLocked(), nil
	}

	process, err := os.FindProcess(record.PID)
	if err != nil {
		err = fmt.Errorf("find frpc process: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	if err := process.Kill(); err != nil {
		err = fmt.Errorf("stop frpc: %w", err)
		m.lastError = err.Error()
		return m.statusLocked(), err
	}
	_ = os.Remove(m.pidPath)
	m.cmd = nil
	m.lastError = ""
	return m.statusLocked(), nil
}

func (m *Manager) Restart(executable, configPath string) (State, error) {
	if _, err := m.Stop(); err != nil {
		return m.Status(), err
	}
	return m.Start(executable, configPath)
}

func (m *Manager) Validate(executable, configPath string) (string, error) {
	resolvedExecutable, err := resolveExecutable(executable)
	if err != nil {
		return "", err
	}
	absoluteConfig, err := filepath.Abs(strings.TrimSpace(configPath))
	if err != nil {
		return "", fmt.Errorf("resolve frpc config path: %w", err)
	}
	cmd := exec.Command(resolvedExecutable, "verify", "-c", absoluteConfig)
	prepareCommand(cmd)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("frpc config verification failed: %s", text)
	}
	if text == "" {
		text = "配置验证通过"
	}
	return text, nil
}

func (m *Manager) wait(cmd *exec.Cmd, pid int) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.readRecord()
	if ok && record.PID == pid {
		_ = os.Remove(m.pidPath)
	}
	if m.cmd == cmd {
		m.cmd = nil
	}
	if err != nil {
		m.lastError = "frpc exited: " + err.Error()
	}
}

func (m *Manager) statusLocked() State {
	state := State{LastError: m.lastError, LogTail: tailLines(m.logPath, 120)}
	record, ok := m.readRecord()
	if !ok {
		return state
	}
	if !processAlive(record.PID, record.Executable) {
		_ = os.Remove(m.pidPath)
		return state
	}
	state.Running = true
	state.PID = record.PID
	state.Executable = record.Executable
	state.ConfigPath = record.ConfigPath
	state.StartedAt = record.StartedAt
	state.Detached = m.cmd == nil || m.cmd.Process == nil || m.cmd.Process.Pid != record.PID
	return state
}

func (m *Manager) writeRecord(record pidRecord) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp := m.pidPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.pidPath)
}

func (m *Manager) readRecord() (pidRecord, bool) {
	data, err := os.ReadFile(m.pidPath)
	if err != nil {
		return pidRecord{}, false
	}
	var record pidRecord
	if json.Unmarshal(data, &record) != nil || record.PID <= 0 || strings.TrimSpace(record.Executable) == "" {
		_ = os.Remove(m.pidPath)
		return pidRecord{}, false
	}
	return record, true
}

func resolveExecutable(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("frpc path is required")
	}
	if strings.ContainsAny(value, `/\`) || filepath.IsAbs(value) {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve frpc executable path: %w", err)
		}
		info, err := os.Stat(absolute)
		if err != nil || info.IsDir() {
			if err == nil {
				err = errors.New("path is a directory")
			}
			return "", fmt.Errorf("frpc executable is not readable: %w", err)
		}
		return absolute, nil
	}
	resolved, err := exec.LookPath(value)
	if err != nil {
		return "", fmt.Errorf("find %s in PATH: %w", value, err)
	}
	return filepath.Abs(resolved)
}

func tailLines(path string, maxLines int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	const maxBytes = 128 * 1024
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

var _ = strconv.Itoa
