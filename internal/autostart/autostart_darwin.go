//go:build darwin

package autostart

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

const macLaunchAgentName = "com.wanstu.frp-client-manager.plist"

func supported() bool { return true }

func macLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", macLaunchAgentName), nil
}

func macLaunchAgent() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	executableXML := html.EscapeString(filepath.Clean(absolute))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.wanstu.frp-client-manager</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>--autostart</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>
`, executableXML), nil
}

func enabled() (bool, error) {
	path, err := macLaunchAgentPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read macOS LaunchAgent: %w", err)
	}
	expected, err := macLaunchAgent()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == strings.TrimSpace(expected), nil
}

func set(value bool) error {
	path, err := macLaunchAgentPath()
	if err != nil {
		return err
	}
	if !value {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove macOS LaunchAgent: %w", err)
		}
		return nil
	}
	content, err := macLaunchAgent()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create macOS LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write macOS LaunchAgent: %w", err)
	}
	return nil
}
