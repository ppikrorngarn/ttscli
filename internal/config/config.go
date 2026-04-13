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
