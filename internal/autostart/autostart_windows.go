//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	registryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName    = "frp-client-manager"
)

func supported() bool { return true }

func command() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return `"` + filepath.Clean(absolute) + `" --autostart`, nil
}

func enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open Windows startup registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(valueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Windows startup value: %w", err)
	}
	expected, err := command()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(value), expected), nil
}

func set(value bool) error {
	if !value {
		key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open Windows startup registry key: %w", err)
		}
		defer key.Close()
		if err := key.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("remove Windows startup value: %w", err)
		}
		return nil
	}

	startCommand, err := command()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows startup registry key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(valueName, startCommand); err != nil {
		return fmt.Errorf("write Windows startup value: %w", err)
	}
	return nil
}
