//go:build !windows && !linux && !darwin

package autostart

func supported() bool        { return false }
func enabled() (bool, error) { return false, nil }
func set(bool) error         { return nil }
