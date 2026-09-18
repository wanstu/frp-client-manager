package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appDirName = "frp-client-manager"

type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
	AutoStart  bool   `json:"auto_start"`
}

type Settings struct {
	FRPCPath        string    `json:"frpc_path"`
	ActiveProfileID string    `json:"active_profile_id"`
	Profiles        []Profile `json:"profiles"`

	// Legacy single-profile fields. They are read for migration and cleared on save.
	ConfigPath        string `json:"config_path,omitempty"`
	StartFRPCOnLaunch bool   `json:"start_frpc_on_launch,omitempty"`
}

type Store struct {
	dir  string
	path string
}

func NewStore() (*Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}
	dir := filepath.Join(root, appDirName)
	return &Store{dir: dir, path: filepath.Join(dir, "settings.json")}, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Load() (Settings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s.defaults(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}

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

	normalizeSettings(&settings)
	if err := validateSettings(settings); err != nil {
		return Settings{}, fmt.Errorf("invalid settings: %w", err)
	}
	return settings, nil
}

func (s *Store) Save(settings Settings) error {
	normalizeSettings(&settings)
	settings.ConfigPath = ""
	settings.StartFRPCOnLaunch = false
	if err := validateSettings(settings); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
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
	frpcPath := "frpc.exe"
	configPath := filepath.Join(s.dir, "frpc.toml")
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidateFRPC := filepath.Join(exeDir, "frpc.exe")
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
	}
}

func normalizeSettings(settings *Settings) {
	settings.FRPCPath = strings.TrimSpace(settings.FRPCPath)
	if settings.ActiveProfileID == "" && len(settings.Profiles) > 0 {
		settings.ActiveProfileID = settings.Profiles[0].ID
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
