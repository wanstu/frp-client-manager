package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wanstu/wails-desktop-kit/jsonstore"
	kitpaths "github.com/wanstu/wails-desktop-kit/paths"
	kittheme "github.com/wanstu/wails-desktop-kit/theme"
)

const appDirName = "frp-client-manager"

type ThemeSettings struct {
	Mode    string `json:"mode"`
	Variant string `json:"variant"`
}

type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
	AutoStart  bool   `json:"auto_start"`
}

type Settings struct {
	FRPCPath        string        `json:"frpc_path"`
	ActiveProfileID string        `json:"active_profile_id"`
	Profiles        []Profile     `json:"profiles"`
	Theme           ThemeSettings `json:"theme"`

	// Legacy single-profile fields. They are read for migration and cleared on save.
	ConfigPath        string `json:"config_path,omitempty"`
	StartFRPCOnLaunch bool   `json:"start_frpc_on_launch,omitempty"`
}

type Store struct {
	dir  string
	path string
}

func (s *Store) settingsStore() *jsonstore.Store[Settings] {
	return jsonstore.New(s.path, jsonstore.Options[Settings]{
		Normalize: func(settings *Settings) {
			defaults := s.defaults()
			if strings.TrimSpace(settings.FRPCPath) == "" {
				settings.FRPCPath = defaults.FRPCPath
			}
			if len(settings.Profiles) == 0 {
				path := strings.TrimSpace(settings.ConfigPath)
				if path == "" {
					path = defaults.Profiles[0].ConfigPath
				}
				settings.Profiles = []Profile{{
					ID:         "default",
					Name:       "默认连接",
					ConfigPath: path,
					AutoStart:  settings.StartFRPCOnLaunch,
				}}
				settings.ActiveProfileID = "default"
			}
			normalizeSettings(settings)
		},
		Validate: validateSettings,
	})
}

func NewStore() (*Store, error) {
	dir, err := kitpaths.EnsureConfigDir(appDirName)
	if err != nil {
		return nil, err
	}
	if err := migrateLegacyConfig(dir); err != nil {
		return nil, err
	}
	return &Store{dir: dir, path: filepath.Join(dir, "settings.json")}, nil
}

func migrateLegacyConfig(targetDir string) error {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	legacyDir := filepath.Join(root, appDirName)
	if samePath(legacyDir, targetDir) {
		return nil
	}
	legacySettings := filepath.Join(legacyDir, "settings.json")
	if _, err := os.Stat(filepath.Join(targetDir, "settings.json")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect target settings: %w", err)
	}
	if _, err := os.Stat(legacySettings); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect legacy settings: %w", err)
	}

	legacyStore := &Store{dir: legacyDir, path: legacySettings}
	settings, err := legacyStore.Load()
	if err != nil {
		return fmt.Errorf("load legacy settings: %w", err)
	}

	settings.FRPCPath, err = migrateManagedPath(settings.FRPCPath, legacyDir, targetDir)
	if err != nil {
		return fmt.Errorf("migrate frpc path: %w", err)
	}
	for i := range settings.Profiles {
		settings.Profiles[i].ConfigPath, err = migrateManagedPath(settings.Profiles[i].ConfigPath, legacyDir, targetDir)
		if err != nil {
			return fmt.Errorf("migrate profile %q config: %w", settings.Profiles[i].Name, err)
		}
	}
	if err := migrateRuntimeState(legacyDir, targetDir); err != nil {
		return fmt.Errorf("migrate runtime state: %w", err)
	}

	target := &Store{dir: targetDir, path: filepath.Join(targetDir, "settings.json")}
	if err := target.Save(settings); err != nil {
		return fmt.Errorf("write migrated settings: %w", err)
	}
	return nil
}

func migrateManagedPath(path, sourceDir, targetDir string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return path, nil
	}
	rel, err := filepath.Rel(sourceDir, path)
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path, nil
	}
	target := filepath.Join(targetDir, rel)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return path, nil
	}
	if _, err := kitpaths.MigrateFileIfMissing(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func migrateRuntimeState(sourceDir, targetDir string) error {
	for _, name := range []string{"frpc.pid.json", "frpc.log", "instances"} {
		if _, err := kitpaths.MigrateTreeMissing(filepath.Join(sourceDir, name), filepath.Join(targetDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func samePath(a, b string) bool {
	return kitpaths.SamePath(a, b)
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Load() (Settings, error) {
	settings, err := s.settingsStore().Load()
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}
	return settings, nil
}

func (s *Store) Save(settings Settings) error {
	settings.ConfigPath = ""
	settings.StartFRPCOnLaunch = false
	if err := s.settingsStore().Save(settings); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

func (s *Store) ActiveProfile(settings Settings) (Profile, error) {
	for _, profile := range settings.Profiles {
		if profile.ID == settings.ActiveProfileID {
			return profile, nil
		}
	}
	return Profile{}, errors.New("active profile not found")
}

func (s *Store) Profile(settings Settings, id string) (Profile, error) {
	id = strings.TrimSpace(id)
	for _, profile := range settings.Profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("profile %q not found", id)
}

func (s *Store) NewProfileID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return "profile-" + hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("profile-%x", time.Now().UnixNano())
}

func (s *Store) defaults() Settings {
	frpcBinary := "frpc"
	if runtime.GOOS == "windows" {
		frpcBinary = "frpc.exe"
	}
	frpcPath := frpcBinary
	configPath := filepath.Join(s.dir, "frpc.toml")
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidateFRPC := filepath.Join(exeDir, frpcBinary)
		if _, err := os.Stat(candidateFRPC); err == nil {
			frpcPath = candidateFRPC
		}
		candidateConfig := filepath.Join(exeDir, "frpc.toml")
		if _, err := os.Stat(candidateConfig); err == nil {
			configPath = candidateConfig
		}
	}
	return Settings{
		FRPCPath:        frpcPath,
		ActiveProfileID: "default",
		Profiles: []Profile{{
			ID:         "default",
			Name:       "默认连接",
			ConfigPath: configPath,
		}},
		Theme: ThemeSettings{Mode: "light", Variant: "aurora"},
	}
}

func normalizeSettings(settings *Settings) {
	settings.FRPCPath = strings.TrimSpace(settings.FRPCPath)
	if settings.ActiveProfileID == "" && len(settings.Profiles) > 0 {
		settings.ActiveProfileID = settings.Profiles[0].ID
	}
	if settings.Theme.Mode == "" {
		settings.Theme.Mode = "light"
	}
	if settings.Theme.Variant == "" {
		settings.Theme.Variant = "aurora"
	}
	for i := range settings.Profiles {
		settings.Profiles[i].ID = strings.TrimSpace(settings.Profiles[i].ID)
		settings.Profiles[i].Name = strings.TrimSpace(settings.Profiles[i].Name)
		settings.Profiles[i].ConfigPath = strings.TrimSpace(settings.Profiles[i].ConfigPath)
		if settings.Profiles[i].Name == "" {
			settings.Profiles[i].Name = "连接 " + fmt.Sprint(i+1)
		}
	}
}

func validateSettings(settings Settings) error {
	if settings.FRPCPath == "" {
		return errors.New("frpc path is required")
	}
	if err := kittheme.ValidateMode(kittheme.Mode(settings.Theme.Mode)); err != nil {
		return fmt.Errorf("invalid theme mode %q: %w", settings.Theme.Mode, err)
	}
	if err := kittheme.ValidatePackName(settings.Theme.Variant); err != nil {
		return fmt.Errorf("invalid theme variant %q: %w", settings.Theme.Variant, err)
	}
	if len(settings.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	ids := make(map[string]struct{}, len(settings.Profiles))
	activeFound := false
	for i, profile := range settings.Profiles {
		if profile.ID == "" {
			return fmt.Errorf("profile %d id is required", i+1)
		}
		if _, ok := ids[profile.ID]; ok {
			return fmt.Errorf("duplicate profile id %q", profile.ID)
		}
		ids[profile.ID] = struct{}{}
		if profile.ConfigPath == "" {
			return fmt.Errorf("profile %q config path is required", profile.Name)
		}
		if profile.ID == settings.ActiveProfileID {
			activeFound = true
		}
	}
	if !activeFound {
		return errors.New("active profile not found")
	}
	return nil
}
