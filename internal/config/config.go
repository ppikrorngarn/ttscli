package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName           = "ttscli"
	configName           = "config.json"
	dirPermission        = 0o700
	filePermission       = 0o600
	CurrentSchemaVersion = 1
)

type Profile struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Credentials map[string]interface{} `json:"credentials"`
	Defaults    map[string]string      `json:"defaults"`
}

type Config struct {
	SchemaVersion  int                `json:"schemaVersion"`
	ActiveProvider string             `json:"activeProvider"`
	ActiveProfile  string             `json:"activeProfile"`
	Profiles       map[string]Profile `json:"profiles"`
}

var (
	userConfigDir  = os.UserConfigDir
	executablePath = os.Executable
	fileExists     = func(path string) (bool, error) {
		_, err := os.Stat(path)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	readFile   = os.ReadFile
	writeFile  = os.WriteFile
	mkdirAll   = os.MkdirAll
	removeFile = os.Remove
	renameFile = os.Rename
)

func Path() (string, error) {
	localPath, err := localPath()
	if err == nil {
		exists, err := fileExists(localPath)
		if err != nil {
			return "", fmt.Errorf("check local config file: %w", err)
		}
		if exists {
			return localPath, nil
		}
	}

	return userPath()
}

func localPath() (string, error) {
	exePath, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), configName), nil
}

func userPath() (string, error) {
	baseDir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(baseDir, appDirName, configName), nil
}

func LoadConfig() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	raw, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{Profiles: make(map[string]Profile)}, nil
		}
		return Config{}, fmt.Errorf("read config file (%s): %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file (%s): %w", path, err)
	}
	if cfg.SchemaVersion == 0 {
		cfg.SchemaVersion = CurrentSchemaVersion
	}
	if cfg.SchemaVersion > CurrentSchemaVersion {
		return Config{}, fmt.Errorf("config schema version %d is newer than supported (%d); please upgrade ttscli", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	path, err := Path()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := mkdirAll(filepath.Dir(path), dirPermission); err != nil {
		return fmt.Errorf("create config dir (%s): %w", filepath.Dir(path), err)
	}

	cfg.SchemaVersion = CurrentSchemaVersion
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	raw = append(raw, '\n')

	tmpPath := path + ".tmp"
	if err := writeFile(tmpPath, raw, filePermission); err != nil {
		return fmt.Errorf("write config file (%s): %w", tmpPath, err)
	}
	if err := renameFile(tmpPath, path); err != nil {
		_ = removeFile(tmpPath)
		return fmt.Errorf("finalize config file (%s): %w", path, err)
	}
	return nil
}

func GetProfile(cfg Config, profileKey string) (Profile, error) {
	profile, ok := cfg.Profiles[profileKey]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", profileKey)
	}
	return profile, nil
}
