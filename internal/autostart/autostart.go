package autostart

func Supported() bool { return supported() }

func Enabled() (bool, error) { return enabled() }

func Set(value bool) error { return set(value) }
