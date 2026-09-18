//go:build windows || linux || darwin

package main

import (
	"context"
	"runtime"
	"sync"

	"github.com/gogpu/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const traySupported = true

type trayManager struct {
	app  *App
	icon []byte

	mu       sync.RWMutex
	ctx      context.Context
	tray     *systray.SystemTray
	started  bool
	stopping bool
}

func newTrayManager(app *App, icon []byte) *trayManager { return &trayManager{app: app, icon: icon} }

func (t *trayManager) Startup(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	t.mu.Unlock()
}

func (t *trayManager) DomReady(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	if t.started {
		t.mu.Unlock()
		return
	}
	t.started = true
	t.stopping = false
	t.mu.Unlock()
	go t.run()
}

func (t *trayManager) Shutdown(context.Context) {
	t.mu.Lock()
	t.stopping = true
	tray := t.tray
	t.mu.Unlock()
	if tray != nil {
		tray.Remove()
	}
}

func (t *trayManager) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tray := systray.New()
	menu := systray.NewMenu()
	menu.Add("显示主窗口", t.ShowWindow)
	menu.Add("隐藏主窗口", t.hideWindow)
	menu.AddSeparator()
	menu.Add("启动全部连接", func() { t.runAction("启动全部连接", t.app.StartAllProfiles) })
	menu.Add("停止全部连接", func() { t.runAction("停止全部连接", t.app.StopAllProfiles) })
	menu.Add("重启全部连接", func() { t.runAction("重启全部连接", t.app.RestartAllProfiles) })
	menu.AddSeparator()

	launchEnabled := false
	launchSupported := false
	if state, err := t.app.GetState(); err == nil {
		launchEnabled = state.LaunchAtLogin
		launchSupported = state.LaunchAtLoginSupported
	}
	var launchItem *systray.MenuItem
	launchItem = menu.AddCheckbox("开机启动管理器", launchEnabled, func() {
		state, err := t.app.GetState()
		if err != nil {
			t.showError("读取开机启动状态失败", err)
			return
		}
		next, err := t.app.SetLaunchAtLogin(!state.LaunchAtLogin)
		if err != nil {
			t.showError("设置开机启动失败", err)
			return
		}
		launchItem.SetChecked(next.LaunchAtLogin)
	})
	if !launchSupported {
		launchItem.SetDisabled(true)
	}

	menu.AddSeparator()
	menu.Add("退出（保留 frpc）", t.quitKeepFRPC)
	menu.Add("退出并停止 frpc", t.quitAndStopFRPC)

	tray.SetIcon(t.icon).SetTooltip("FRP Client Manager").SetMenu(menu)
	tray.OnClick(t.ShowWindow)
	tray.OnDoubleClick(t.ShowWindow)
	tray.Show()

	t.mu.Lock()
	if t.stopping {
		t.started = false
		t.mu.Unlock()
		tray.Remove()
		return
	}
	t.tray = tray
	t.mu.Unlock()

	_ = tray.Run()

	t.mu.Lock()
	t.tray = nil
	t.started = false
	t.mu.Unlock()
}

func (t *trayManager) ShowWindow() {
	if ctx := t.runtimeContext(); ctx != nil {
		wailsruntime.WindowShow(ctx)
		wailsruntime.WindowUnminimise(ctx)
	}
}

func (t *trayManager) hideWindow() {
	if ctx := t.runtimeContext(); ctx != nil {
		wailsruntime.WindowHide(ctx)
	}
}

func (t *trayManager) quitKeepFRPC() {
	if ctx := t.runtimeContext(); ctx != nil {
		wailsruntime.Quit(ctx)
	}
}

func (t *trayManager) quitAndStopFRPC() {
	if _, err := t.app.StopAllProfiles(); err != nil {
		t.showError("停止全部连接失败，未退出", err)
		return
	}
	t.quitKeepFRPC()
}

func (t *trayManager) runAction(title string, action func() (UIState, error)) {
	if _, err := action(); err != nil {
		t.showError(title+"失败", err)
	}
}

func (t *trayManager) showError(title string, err error) {
	t.ShowWindow()
	if ctx := t.runtimeContext(); ctx != nil {
		_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
			Type:    wailsruntime.ErrorDialog,
			Title:   title,
			Message: err.Error(),
		})
	}
}

func (t *trayManager) runtimeContext() context.Context {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ctx
}
