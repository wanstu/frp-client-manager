//go:build !windows && !linux && !darwin

package main

import (
	"context"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const traySupported = false

type trayManager struct {
	mu  sync.RWMutex
	ctx context.Context
}

func newTrayManager(*App, []byte) *trayManager { return &trayManager{} }

func (t *trayManager) Startup(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	t.mu.Unlock()
}

func (t *trayManager) DomReady(context.Context) {}
func (t *trayManager) Shutdown(context.Context) {}

func (t *trayManager) ShowWindow() {
	t.mu.RLock()
	ctx := t.ctx
	t.mu.RUnlock()
	if ctx != nil {
		wailsruntime.WindowShow(ctx)
		wailsruntime.WindowUnminimise(ctx)
	}
}
