package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	desktopkit "github.com/wanstu/wails-desktop-kit"
	kitui "github.com/wanstu/wails-desktop-kit/ui"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

//go:embed assets/appicon.png
var appIcon []byte

func main() {
	launch, err := desktopkit.ParseLaunchOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
	app, err := NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
	if err := runDesktop(app, launch); err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
}

func runDesktop(app *App, launch desktopkit.LaunchOptions) error {
	assets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		return err
	}

	window := desktopkit.DefaultWindowConfig()
	window.Width = 1080
	window.Height = 720
	window.MinWidth = 860
	window.MinHeight = 600
	window.HidePolicy = desktopkit.HideAlways
	window.StartHiddenOnAutoStart = true
	window.Background = desktopkit.Color{R: 245, G: 247, B: 250, A: 1}

	startAll := desktopkit.Action("启动全部连接", func(*desktopkit.Controller) error {
		_, err := app.StartAllProfiles()
		return err
	})
	stopAll := desktopkit.Action("停止全部连接", func(*desktopkit.Controller) error {
		_, err := app.StopAllProfiles()
		return err
	})
	restartAll := desktopkit.Action("重启全部连接", func(*desktopkit.Controller) error {
		_, err := app.RestartAllProfiles()
		return err
	})

	quitKeep := desktopkit.Action("退出（保留 frpc）", func(controller *desktopkit.Controller) error {
		controller.Quit()
		return nil
	})
	quitStop := desktopkit.Action("退出并停止 frpc", func(controller *desktopkit.Controller) error {
		if _, err := app.StopAllProfiles(); err != nil {
			return err
		}
		controller.Quit()
		return nil
	})
	quitStop.ErrorTitle = "停止全部连接失败，未退出"

	return desktopkit.Run(desktopkit.Config{
		ID:                   "frp-client-manager-v1",
		Title:                "FRP Client Manager",
		Assets:               kitui.Mount(assets),
		Bind:                 []interface{}{app},
		Theme:                desktopkit.DefaultThemeConfig(),
		Launch:               launch,
		Window:               window,
		SingleInstance:       true,
		SecondInstancePolicy: desktopkit.SecondInstanceWakeManual,
		Tray: desktopkit.TrayConfig{
			Enabled:            true,
			Icon:               appIcon,
			AutoStart:          app.launchAtLogin,
			LaunchAtLoginLabel: "开机启动管理器",
			Items:              []desktopkit.TrayItem{startAll, stopAll, restartAll},
			FooterItems:        []desktopkit.TrayItem{quitKeep, quitStop},
			DisableQuit:        true,
		},
		Hooks: desktopkit.Hooks{
			Ready:    app.setController,
			Startup:  app.startup,
			Shutdown: app.shutdown,
		},
	})
}
