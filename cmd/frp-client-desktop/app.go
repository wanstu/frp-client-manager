package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"frp-client-manager/internal/config"
	"frp-client-manager/internal/frpc"
	"frp-client-manager/internal/frpconfig"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	kitautostart "github.com/wanstu/wails-desktop-kit/autostart"
)

type ProfileState struct {
	Profile config.Profile `json:"profile"`
	Process frpc.State     `json:"process"`
}

type UIState struct {
	Settings               config.Settings `json:"settings"`
	Process                frpc.State      `json:"process"`
	Profiles               []ProfileState  `json:"profiles"`
	RunningCount           int             `json:"running_count"`
	LaunchAtLoginSupported bool            `json:"launch_at_login_supported"`
	LaunchAtLogin          bool            `json:"launch_at_login"`
	DataDir                string          `json:"data_dir"`
	StartupError           string          `json:"startup_error"`
}

type VisualConfigCatalog struct {
	ProxyTypes         []string `json:"proxy_types"`
	VisitorTypes       []string `json:"visitor_types"`
	PluginTypes        []string `json:"plugin_types"`
	TransportProtocols []string `json:"transport_protocols"`
	AuthMethods        []string `json:"auth_methods"`
	HealthCheckTypes   []string `json:"health_check_types"`
}

type App struct {
	store         *config.Store
	launchAtLogin *kitautostart.Manager

	mu           sync.RWMutex
	ctx          context.Context
	startupError string
	managers     map[string]*frpc.Manager
}

func NewApp() (*App, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	launchAtLogin, err := kitautostart.New(kitautostart.Config{
		ID:          "frp-client-manager",
		DisplayName: "FRP Client Manager",
		Comment:     "Manage multiple frpc connections",
		Arguments:   []string{"--autostart"},
	})
	if err != nil {
		return nil, err
	}
	return &App{
		store:         store,
		launchAtLogin: launchAtLogin,
		managers:      make(map[string]*frpc.Manager),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	settings, err := a.store.Load()
	if err != nil {
		a.setStartupError(err)
		return
	}

	var failures []string
	for _, profile := range settings.Profiles {
		if !profile.AutoStart {
			continue
		}
		if _, err := a.manager(profile.ID).Start(settings.FRPCPath, profile.ConfigPath); err != nil {
			failures = append(failures, profile.Name+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		a.setStartupError(errors.New(strings.Join(failures, "; ")))
	} else {
		a.setStartupError(nil)
	}
}

func (a *App) shutdown(context.Context) {}

func (a *App) GetState() (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	launchEnabled, err := a.launchAtLogin.Enabled()
	if err != nil {
		return UIState{}, err
	}

	profiles := make([]ProfileState, 0, len(settings.Profiles))
	runningCount := 0
	var activeProcess frpc.State
	for _, profile := range settings.Profiles {
		process := a.manager(profile.ID).Status()
		if process.Running {
			runningCount++
		}
		profiles = append(profiles, ProfileState{Profile: profile, Process: process})
		if profile.ID == settings.ActiveProfileID {
			activeProcess = process
		}
	}

	a.mu.RLock()
	startupError := a.startupError
	a.mu.RUnlock()

	return UIState{
		Settings:               settings,
		Process:                activeProcess,
		Profiles:               profiles,
		RunningCount:           runningCount,
		LaunchAtLoginSupported: a.launchAtLogin.Supported(),
		LaunchAtLogin:          launchEnabled,
		DataDir:                a.store.Dir(),
		StartupError:           startupError,
	}, nil
}

func (a *App) SaveSettings(settings config.Settings) (UIState, error) {
	current, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	if strings.TrimSpace(settings.FRPCPath) == "" {
		settings.FRPCPath = current.FRPCPath
	}
	if len(settings.Profiles) == 0 {
		settings.Profiles = current.Profiles
	}
	if settings.ActiveProfileID == "" {
		settings.ActiveProfileID = current.ActiveProfileID
	}
	if err := a.store.Save(settings); err != nil {
		return UIState{}, err
	}
	return a.GetState()
}

func (a *App) SetLaunchAtLogin(value bool) (UIState, error) {
	if value && !a.launchAtLogin.Supported() {
		return UIState{}, errors.New("当前平台不支持开机启动")
	}
	if err := a.launchAtLogin.SetEnabled(value); err != nil {
		return UIState{}, err
	}
	if ctx := a.runtimeContext(); ctx != nil {
		wailsruntime.EventsEmit(ctx, "desktop:preferences-changed")
	}
	return a.GetState()
}

func (a *App) CreateProfile(name, configPath string, autoStart bool) (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	name = strings.TrimSpace(name)
	configPath = strings.TrimSpace(configPath)
	if name == "" {
		name = fmt.Sprintf("连接 %d", len(settings.Profiles)+1)
	}
	if configPath == "" {
		return UIState{}, errors.New("请选择配置文件")
	}
	for _, existing := range settings.Profiles {
		if strings.EqualFold(existing.Name, name) {
			return UIState{}, fmt.Errorf("连接名称 %q 已存在", name)
		}
		if samePath(existing.ConfigPath, configPath) {
			return UIState{}, fmt.Errorf("配置文件已由 %q 管理", existing.Name)
		}
	}
	id := a.store.NewProfileID()
	for a.profileIDExists(settings, id) {
		id = a.store.NewProfileID()
	}
	settings.Profiles = append(settings.Profiles, config.Profile{
		ID:         id,
		Name:       name,
		ConfigPath: configPath,
		AutoStart:  autoStart,
	})
	settings.ActiveProfileID = id
	if err := a.store.Save(settings); err != nil {
		return UIState{}, err
	}
	return a.GetState()
}

func (a *App) UpdateProfile(profile config.Profile) (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.ConfigPath = strings.TrimSpace(profile.ConfigPath)
	if profile.ID == "" || profile.Name == "" || profile.ConfigPath == "" {
		return UIState{}, errors.New("连接 ID、名称和配置文件不能为空")
	}
	found := false
	for i, existing := range settings.Profiles {
		if existing.ID == profile.ID {
			settings.Profiles[i] = profile
			found = true
			continue
		}
		if strings.EqualFold(existing.Name, profile.Name) {
			return UIState{}, fmt.Errorf("连接名称 %q 已存在", profile.Name)
		}
		if samePath(existing.ConfigPath, profile.ConfigPath) {
			return UIState{}, fmt.Errorf("配置文件已由 %q 管理", existing.Name)
		}
	}
	if !found {
		return UIState{}, fmt.Errorf("连接 %q 不存在", profile.ID)
	}
	if err := a.store.Save(settings); err != nil {
		return UIState{}, err
	}
	return a.GetState()
}

func (a *App) DeleteProfile(id string) (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	if len(settings.Profiles) <= 1 {
		return UIState{}, errors.New("至少需要保留一个连接配置")
	}
	profile, err := a.store.Profile(settings, id)
	if err != nil {
		return UIState{}, err
	}
	if a.manager(id).Status().Running {
		return UIState{}, fmt.Errorf("连接 %q 正在运行，请先停止后再删除", profile.Name)
	}
	next := make([]config.Profile, 0, len(settings.Profiles)-1)
	for _, item := range settings.Profiles {
		if item.ID != id {
			next = append(next, item)
		}
	}
	settings.Profiles = next
	if settings.ActiveProfileID == id {
		settings.ActiveProfileID = settings.Profiles[0].ID
	}
	if err := a.store.Save(settings); err != nil {
		return UIState{}, err
	}
	a.mu.Lock()
	delete(a.managers, id)
	a.mu.Unlock()
	return a.GetState()
}

func (a *App) SetActiveProfile(id string) (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	if _, err := a.store.Profile(settings, id); err != nil {
		return UIState{}, err
	}
	settings.ActiveProfileID = id
	if err := a.store.Save(settings); err != nil {
		return UIState{}, err
	}
	return a.GetState()
}

func (a *App) StartProfile(id string) (UIState, error) {
	settings, profile, err := a.profileSettings(id)
	if err != nil {
		return UIState{}, err
	}
	if _, err := a.manager(profile.ID).Start(settings.FRPCPath, profile.ConfigPath); err != nil {
		state, _ := a.GetState()
		return state, err
	}
	a.setStartupError(nil)
	return a.GetState()
}

func (a *App) StopProfile(id string) (UIState, error) {
	settings, profile, err := a.profileSettings(id)
	if err != nil {
		return UIState{}, err
	}
	_ = settings
	if _, err := a.manager(profile.ID).Stop(); err != nil {
		state, _ := a.GetState()
		return state, err
	}
	return a.GetState()
}

func (a *App) RestartProfile(id string) (UIState, error) {
	settings, profile, err := a.profileSettings(id)
	if err != nil {
		return UIState{}, err
	}
	if _, err := a.manager(profile.ID).Restart(settings.FRPCPath, profile.ConfigPath); err != nil {
		state, _ := a.GetState()
		return state, err
	}
	a.setStartupError(nil)
	return a.GetState()
}

func (a *App) StartAllProfiles() (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	var failures []string
	for _, profile := range settings.Profiles {
		if _, err := a.manager(profile.ID).Start(settings.FRPCPath, profile.ConfigPath); err != nil {
			failures = append(failures, profile.Name+": "+err.Error())
		}
	}
	state, _ := a.GetState()
	if len(failures) > 0 {
		return state, errors.New(strings.Join(failures, "; "))
	}
	return state, nil
}

func (a *App) StopAllProfiles() (UIState, error) {
	settings, err := a.store.Load()
	if err != nil {
		return UIState{}, err
	}
	var failures []string
	for _, profile := range settings.Profiles {
		if _, err := a.manager(profile.ID).Stop(); err != nil {
			failures = append(failures, profile.Name+": "+err.Error())
		}
	}
	state, _ := a.GetState()
	if len(failures) > 0 {
		return state, errors.New(strings.Join(failures, "; "))
	}
	return state, nil
}

func (a *App) RestartAllProfiles() (UIState, error) {
	if _, err := a.StopAllProfiles(); err != nil {
		state, _ := a.GetState()
		return state, err
	}
	return a.StartAllProfiles()
}

// Legacy/active-profile actions retained for the existing UI surfaces.
func (a *App) StartFRPC() (UIState, error) {
	id, err := a.activeProfileID()
	if err != nil {
		return UIState{}, err
	}
	return a.StartProfile(id)
}

func (a *App) StopFRPC() (UIState, error) {
	id, err := a.activeProfileID()
	if err != nil {
		return UIState{}, err
	}
	return a.StopProfile(id)
}

func (a *App) RestartFRPC() (UIState, error) {
	id, err := a.activeProfileID()
	if err != nil {
		return UIState{}, err
	}
	return a.RestartProfile(id)
}

func (a *App) ValidateConfig() (string, error) {
	settings, profile, err := a.activeSettings()
	if err != nil {
		return "", err
	}
	return a.manager(profile.ID).Validate(settings.FRPCPath, profile.ConfigPath)
}

func (a *App) ReadConfig() (string, error) {
	_, profile, err := a.activeSettings()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(profile.ConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 frpc 配置失败: %w", err)
	}
	return string(data), nil
}

func (a *App) SaveConfig(content string) (UIState, error) {
	_, profile, err := a.activeSettings()
	if err != nil {
		return UIState{}, err
	}
	path := strings.TrimSpace(profile.ConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return UIState{}, fmt.Errorf("创建配置目录失败: %w", err)
	}
	if current, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", current, 0o600)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return UIState{}, fmt.Errorf("保存配置失败: %w", err)
	}
	return a.GetState()
}

func (a *App) GetVisualConfig() (frpconfig.ClientConfig, error) {
	_, profile, err := a.activeSettings()
	if err != nil {
		return frpconfig.ClientConfig{}, err
	}
	return frpconfig.Load(profile.ConfigPath)
}

func (a *App) GetVisualConfigCatalog() VisualConfigCatalog {
	return VisualConfigCatalog{
		ProxyTypes:         frpconfig.ProxyTypes(),
		VisitorTypes:       frpconfig.VisitorTypes(),
		PluginTypes:        frpconfig.PluginTypes(),
		TransportProtocols: []string{"tcp", "kcp", "quic", "websocket", "wss"},
		AuthMethods:        []string{"token", "oidc"},
		HealthCheckTypes:   []string{"", "tcp", "http"},
	}
}

func (a *App) PreviewVisualConfig(cfg frpconfig.ClientConfig) (string, error) {
	data, err := frpconfig.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SaveVisualConfig(cfg frpconfig.ClientConfig) (UIState, error) {
	_, profile, err := a.activeSettings()
	if err != nil {
		return UIState{}, err
	}
	if err := frpconfig.Save(profile.ConfigPath, cfg); err != nil {
		return UIState{}, err
	}
	return a.GetState()
}

func (a *App) ChooseFRPCExecutable() (string, error) {
	ctx := a.runtimeContext()
	if ctx == nil {
		return "", errors.New("桌面运行时尚未就绪")
	}
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 frpc 可执行文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "frpc executable", Pattern: "frpc.exe;frpc"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (a *App) ChooseConfigFile() (string, error) {
	ctx := a.runtimeContext()
	if ctx == nil {
		return "", errors.New("桌面运行时尚未就绪")
	}
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 frpc 配置文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "FRP config", Pattern: "*.toml;*.ini;*.yaml;*.yml;*.json"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (a *App) activeProfileID() (string, error) {
	settings, err := a.store.Load()
	if err != nil {
		return "", err
	}
	if _, err := a.store.ActiveProfile(settings); err != nil {
		return "", err
	}
	return settings.ActiveProfileID, nil
}

func (a *App) activeSettings() (config.Settings, config.Profile, error) {
	settings, err := a.store.Load()
	if err != nil {
		return config.Settings{}, config.Profile{}, err
	}
	profile, err := a.store.ActiveProfile(settings)
	return settings, profile, err
}

func (a *App) profileSettings(id string) (config.Settings, config.Profile, error) {
	settings, err := a.store.Load()
	if err != nil {
		return config.Settings{}, config.Profile{}, err
	}
	profile, err := a.store.Profile(settings, id)
	return settings, profile, err
}

func (a *App) profileIDExists(settings config.Settings, id string) bool {
	for _, profile := range settings.Profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func (a *App) manager(id string) *frpc.Manager {
	a.mu.Lock()
	defer a.mu.Unlock()
	if manager := a.managers[id]; manager != nil {
		return manager
	}
	dir := a.store.Dir()
	if id != "default" {
		sum := sha256.Sum256([]byte(id))
		dir = filepath.Join(dir, "instances", fmt.Sprintf("%x", sum[:8]))
	}
	manager := frpc.NewManager(dir)
	a.managers[id] = manager
	return manager
}

func samePath(a, b string) bool {
	left, errLeft := filepath.Abs(strings.TrimSpace(a))
	right, errRight := filepath.Abs(strings.TrimSpace(b))
	if errLeft == nil && errRight == nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (a *App) runtimeContext() context.Context {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ctx
}

func (a *App) setStartupError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err == nil {
		a.startupError = ""
		return
	}
	a.startupError = err.Error()
}
