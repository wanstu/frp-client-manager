package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"frp-client-manager/internal/frpcdownload"
)

func (a *App) DownloadLatestFRPC() (frpcdownload.Result, error) {
	a.frpcDownloadMu.Lock()
	defer a.frpcDownloadMu.Unlock()

	settings, err := a.store.Load()
	if err != nil {
		return frpcdownload.Result{}, err
	}
	if frpcPathReady(settings.FRPCPath) {
		return frpcdownload.Result{}, errors.New("当前已配置可用的 frpc，无需自动下载")
	}

	base := context.Background()
	if controller := a.runtimeController(); controller != nil {
		if runtimeContext := controller.Context(); runtimeContext != nil {
			base = runtimeContext
		}
	}
	ctx, cancel := context.WithTimeout(base, 5*time.Minute)
	defer cancel()

	downloader := frpcdownload.New(filepath.Join(a.store.Dir(), "frpc"))
	result, err := downloader.DownloadLatest(ctx)
	if err != nil {
		return frpcdownload.Result{}, err
	}

	settings, err = a.store.Load()
	if err != nil {
		return frpcdownload.Result{}, err
	}
	settings.FRPCPath = result.Path
	if err := a.store.Save(settings); err != nil {
		return frpcdownload.Result{}, err
	}
	return result, nil
}

func frpcPathReady(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	if filepath.IsAbs(path) || filepath.Dir(path) != "." {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return false
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			return false
		}
		return true
	}

	resolved, err := exec.LookPath(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return false
	}
	return true
}
