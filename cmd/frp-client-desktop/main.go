package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

//go:embed assets/appicon.png
var appIcon []byte

func main() {
	startHidden, err := launchOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
	app, err := NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
	if err := runDesktop(app, startHidden); err != nil {
		fmt.Fprintln(os.Stderr, "frp-client:", err)
		os.Exit(1)
	}
}

func launchOptions(args []string) (bool, error) {
	flags := flag.NewFlagSet("frp-client", flag.ContinueOnError)
	autostart := flags.Bool("autostart", false, "start hidden after desktop login")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() != 0 {
		return false, fmt.Errorf("only --autostart is supported")
	}
	return *autostart, nil
}

func runDesktop(app *App, startHidden bool) error {
	assets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		return err
	}
	tray := newTrayManager(app, appIcon)
	return wails.Run(&options.App{
		Title:             "FRP Client Manager",
		Width:             1080,
		Height:            720,
		MinWidth:          860,
		MinHeight:         600,
		StartHidden:       startHidden && traySupported,
		HideWindowOnClose: traySupported,
		AssetServer:       &assetserver.Options{Assets: assets},
		BackgroundColour:  &options.RGBA{R: 245, G: 247, B: 250, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			tray.Startup(ctx)
		},
		OnDomReady: tray.DomReady,
		OnShutdown: func(ctx context.Context) {
			tray.Shutdown(ctx)
			app.shutdown(ctx)
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "frp-client-manager-v1",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				tray.ShowWindow()
			},
		},
		Bind: []interface{}{app},
	})
}
