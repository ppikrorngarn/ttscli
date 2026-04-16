package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName    = "ttscli"
	configName    = "config.json"
	dirPermission = 0o755
)

type Defaults struct {
	Voice  string `json:"voice,omitempty"`
	Lang   string `json:"lang,omitempty"`
	APIKey string `json:"apiKey,omitempty"`
}

type Profile struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Credentials map[string]interface{} `json:"credentials"`
	Defaults    map[string]string      `json:"defaults"`
}

type Config struct {
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

func LoadDefaults() (Defaults, error) {
	path, err := Path()
	if err != nil {
		return Defaults{}, err
	}
	raw, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults{}, nil
		}
		return Defaults{}, fmt.Errorf("read config file: %w", err)
	}

	var defaults Defaults
	if err := json.Unmarshal(raw, &defaults); err != nil {
		return Defaults{}, fmt.Errorf("parse config file: %w", err)
	}
	return defaults, nil
}

func SaveDefaults(defaults Defaults) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := mkdirAll(filepath.Dir(path), dirPermission); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	raw = append(raw, '\n')
	if err := writeFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func ClearDefaults() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := removeFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove config file: %w", err)
	}
	return nil
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
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := mkdirAll(filepath.Dir(path), dirPermission); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	raw = append(raw, '\n')
	if err := writeFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
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

func SetProfile(cfg Config, profile Profile) error {
	profileKey := profile.Provider + ":" + profile.Name
	cfg.Profiles[profileKey] = profile
	return SaveConfig(cfg)
}

func DeleteProfile(cfg Config, profileKey string) error {
	if _, ok := cfg.Profiles[profileKey]; !ok {
		return fmt.Errorf("profile %q not found", profileKey)
	}
	delete(cfg.Profiles, profileKey)
	return SaveConfig(cfg)
}
