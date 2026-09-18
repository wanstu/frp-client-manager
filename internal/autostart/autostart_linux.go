//go:build linux

package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const linuxDesktopFileName = "frp-client-manager.desktop"

func supported() bool { return true }

func linuxAutostartPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "autostart", linuxDesktopFileName), nil
}

func linuxDesktopEntry() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	execValue := quoteDesktopExec(filepath.Clean(absolute)) + " --autostart"
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Version=1.0\n" +
		"Name=FRP Client Manager\n" +
		"Comment=Start FRP Client Manager after login\n" +
		"Exec=" + execValue + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n", nil
}

func quoteDesktopExec(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
		"%", "%%",
	)
	return "\"" + replacer.Replace(value) + "\""
}

func enabled() (bool, error) {
	path, err := linuxAutostartPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Linux autostart entry: %w", err)
	}
	expected, err := linuxDesktopEntry()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == strings.TrimSpace(expected), nil
}

func set(value bool) error {
	path, err := linuxAutostartPath()
	if err != nil {
		return err
	}
	if !value {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove Linux autostart entry: %w", err)
		}
		return nil
	}
	entry, err := linuxDesktopEntry()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Linux autostart directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(entry), 0o600); err != nil {
		return fmt.Errorf("write Linux autostart entry: %w", err)
	}
	return nil
}
