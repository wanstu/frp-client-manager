package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigratesLegacySingleProfile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir, path: filepath.Join(dir, "settings.json")}
	legacy := map[string]any{
		"frpc_path":            "C:\\frp\\frpc.exe",
		"config_path":          "C:\\frp\\legacy.toml",
		"start_frpc_on_launch": true,
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(settings.Profiles) != 1 {
		t.Fatalf("profiles = %d, want 1", len(settings.Profiles))
	}
	profile := settings.Profiles[0]
	if profile.ID != "default" || profile.Name != "默认连接" {
		t.Fatalf("unexpected migrated profile: %#v", profile)
	}
	if profile.ConfigPath != "C:\\frp\\legacy.toml" {
		t.Fatalf("config path = %q", profile.ConfigPath)
	}
	if !profile.AutoStart {
		t.Fatal("auto start was not migrated")
	}
	if settings.ActiveProfileID != "default" {
		t.Fatalf("active profile = %q", settings.ActiveProfileID)
	}
}

func TestSaveModernProfilesClearsLegacyFields(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir, path: filepath.Join(dir, "settings.json")}
	settings := Settings{
		FRPCPath:        "frpc.exe",
		ActiveProfileID: "a",
		Profiles: []Profile{
			{ID: "a", Name: "A", ConfigPath: "a.toml", AutoStart: true},
			{ID: "b", Name: "B", ConfigPath: "b.toml"},
		},
		ConfigPath:        "legacy.toml",
		StartFRPCOnLaunch: true,
	}
	if err := store.Save(settings); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["config_path"]; ok {
		t.Fatal("legacy config_path should be omitted")
	}
	if _, ok := saved["start_frpc_on_launch"]; ok {
		t.Fatal("legacy start_frpc_on_launch should be omitted")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Profiles) != 2 || loaded.ActiveProfileID != "a" {
		t.Fatalf("unexpected loaded settings: %#v", loaded)
	}
}

func TestSaveRejectsMissingActiveProfile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir, path: filepath.Join(dir, "settings.json")}
	err := store.Save(Settings{
		FRPCPath:        "frpc.exe",
		ActiveProfileID: "missing",
		Profiles:        []Profile{{ID: "a", Name: "A", ConfigPath: "a.toml"}},
	})
	if err == nil {
		t.Fatal("Save() expected missing active profile error")
	}
}
