package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubConfigDeps() func() {
	oldUserConfigDir := userConfigDir
	oldExecutablePath := executablePath
	oldFileExists := fileExists
	oldReadFile := readFile
	oldWriteFile := writeFile
	oldMkdirAll := mkdirAll
	oldRemoveFile := removeFile
	oldRenameFile := renameFile

	userConfigDir = func() (string, error) {
		return filepath.Join(string(filepath.Separator), "tmp", "usercfg"), nil
	}
	executablePath = func() (string, error) {
		return filepath.Join(string(filepath.Separator), "tmp", "app", "ttscli"), nil
	}
	fileExists = func(path string) (bool, error) {
		return false, nil
	}
	readFile = func(path string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		return nil
	}
	mkdirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	removeFile = func(path string) error {
		return nil
	}
	renameFile = func(oldpath, newpath string) error {
		return nil
	}

	return func() {
		userConfigDir = oldUserConfigDir
		executablePath = oldExecutablePath
		fileExists = oldFileExists
		readFile = oldReadFile
		writeFile = oldWriteFile
		mkdirAll = oldMkdirAll
		removeFile = oldRemoveFile
		renameFile = oldRenameFile
	}
}

// Path tests

func TestPathPrefersLocalConfigNextToBinaryWhenExists(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	localCfg := filepath.Join(string(filepath.Separator), "tmp", "app", configName)
	fileExists = func(path string) (bool, error) {
		return filepath.Clean(path) == filepath.Clean(localCfg), nil
	}

	got, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	want := localCfg
	if got != want {
		t.Fatalf("unexpected path: got=%q want=%q", got, want)
	}
}

func TestPathFallsBackToUserPathWhenLocalConfigMissing(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	got, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	want := filepath.Join(string(filepath.Separator), "tmp", "usercfg", appDirName, configName)
	if got != want {
		t.Fatalf("unexpected path: got=%q want=%q", got, want)
	}
}

// LoadConfig tests

func TestLoadConfigFileNotFoundReturnsEmpty(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	// readFile returns ErrNotExist by default in stub.
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error for missing config file, got: %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("expected Profiles map to be initialized")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty profiles, got: %v", cfg.Profiles)
	}
}

func TestLoadConfigValidJSON(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	raw, _ := json.Marshal(Config{
		ActiveProvider: "gcp",
		ActiveProfile:  "default",
		Profiles: map[string]Profile{
			"gcp:default": {Provider: "gcp", Name: "default"},
		},
	})
	readFile = func(path string) ([]byte, error) { return raw, nil }

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveProvider != "gcp" || cfg.ActiveProfile != "default" {
		t.Errorf("unexpected active profile: %s:%s", cfg.ActiveProvider, cfg.ActiveProfile)
	}
	if _, ok := cfg.Profiles["gcp:default"]; !ok {
		t.Error("expected gcp:default profile to be loaded")
	}
}

func TestLoadConfigNilProfilesInitialized(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) {
		return []byte(`{"activeProvider":"gcp","activeProfile":"default"}`), nil
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("expected Profiles to be initialized even when absent from JSON")
	}
}

func TestLoadConfigAssumesCurrentSchemaVersionWhenMissing(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) {
		return []byte(`{"activeProvider":"gcp","activeProfile":"default","profiles":{}}`), nil
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected schemaVersion to default to %d, got %d", CurrentSchemaVersion, cfg.SchemaVersion)
	}
}

func TestLoadConfigRejectsNewerSchemaVersion(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) {
		return []byte(`{"schemaVersion":99,"profiles":{}}`), nil
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for unsupported schema version")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("expected upgrade-hint error, got: %v", err)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) { return []byte(`{invalid`), nil }

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadConfigReadError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for read failure")
	}
	expectedPath := filepath.FromSlash("/tmp/usercfg/ttscli/config.json")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to include config path, got: %v", err)
	}
}

func TestLoadConfigParseError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(path string) ([]byte, error) {
		return []byte(`{invalid`), nil
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	expectedPath := filepath.FromSlash("/tmp/usercfg/ttscli/config.json")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to include config path, got: %v", err)
	}
}

func TestParseProfileKeyValid(t *testing.T) {
	key, provider, name, err := ParseProfileKey("gcp:default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "gcp:default" || provider != "gcp" || name != "default" {
		t.Fatalf("unexpected parse result: key=%q provider=%q name=%q", key, provider, name)
	}
}

func TestParseProfileKeyInvalid(t *testing.T) {
	tests := []string{"", "gcp", ":default", "gcp:", "gcp:work:extra"}
	for _, raw := range tests {
		_, _, _, err := ParseProfileKey(raw)
		if err == nil {
			t.Fatalf("expected error for %q", raw)
		}
		if !strings.Contains(err.Error(), "invalid profile key") {
			t.Fatalf("expected invalid profile key error for %q, got: %v", raw, err)
		}
	}
}

func TestBuildProfileKeyValid(t *testing.T) {
	key, provider, name, err := BuildProfileKey(" gcp ", " work ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "gcp:work" || provider != "gcp" || name != "work" {
		t.Fatalf("unexpected build result: key=%q provider=%q name=%q", key, provider, name)
	}
}

func TestBuildProfileKeyInvalid(t *testing.T) {
	tests := []struct {
		provider string
		name     string
		want     string
	}{
		{provider: "", name: "work", want: "provider is required"},
		{provider: "gcp", name: "", want: "profile name is required"},
		{provider: "gc:p", name: "work", want: "invalid provider"},
		{provider: "gcp", name: "wo:rk", want: "invalid profile name"},
	}

	for _, tt := range tests {
		_, _, _, err := BuildProfileKey(tt.provider, tt.name)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("expected %q error for provider=%q name=%q, got: %v", tt.want, tt.provider, tt.name, err)
		}
	}
}

func TestGetProfileRejectsInvalidProfileKey(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"gcp:default": {Provider: "gcp", Name: "default"},
		},
	}

	_, err := GetProfile(cfg, "gcp")
	if err == nil || !strings.Contains(err.Error(), "invalid profile key") {
		t.Fatalf("expected invalid profile key error, got: %v", err)
	}
}

// SaveConfig tests

func TestSaveConfigSuccess(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	var writtenPath string
	var writtenData []byte
	var writtenPerm os.FileMode
	var dirPerm os.FileMode
	var renameSrc, renameDst string
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		writtenPath = path
		writtenData = data
		writtenPerm = perm
		return nil
	}
	mkdirAll = func(_ string, perm os.FileMode) error {
		dirPerm = perm
		return nil
	}
	renameFile = func(oldpath, newpath string) error {
		renameSrc = oldpath
		renameDst = newpath
		return nil
	}

	cfg := Config{
		ActiveProvider: "gcp",
		ActiveProfile:  "default",
		Profiles: map[string]Profile{
			"gcp:default": {Provider: "gcp", Name: "default"},
		},
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(writtenPath, ".tmp") {
		t.Errorf("expected writeFile to target a .tmp path, got %q", writtenPath)
	}
	if renameSrc != writtenPath {
		t.Errorf("expected rename from tmp path %q, got src %q", writtenPath, renameSrc)
	}
	if renameDst == "" || strings.HasSuffix(renameDst, ".tmp") {
		t.Errorf("expected rename target to be final config path, got %q", renameDst)
	}
	if renameDst+".tmp" != renameSrc {
		t.Errorf("expected tmp path to be final path + \".tmp\": src=%q dst=%q", renameSrc, renameDst)
	}
	var parsed Config
	if err := json.Unmarshal(writtenData, &parsed); err != nil {
		t.Fatalf("written data is not valid JSON: %v", err)
	}
	if parsed.ActiveProvider != "gcp" {
		t.Errorf("unexpected active provider in saved data: %q", parsed.ActiveProvider)
	}
	if parsed.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected schemaVersion %d in saved data, got %d", CurrentSchemaVersion, parsed.SchemaVersion)
	}
	if writtenPerm != 0o600 {
		t.Errorf("expected config file perm 0o600, got %o", writtenPerm)
	}
	if dirPerm != 0o700 {
		t.Errorf("expected config dir perm 0o700, got %o", dirPerm)
	}
}

func TestSaveConfigMkdirError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	mkdirAll = func(path string, perm os.FileMode) error {
		return errors.New("mkdir failed")
	}

	err := SaveConfig(Config{Profiles: map[string]Profile{}})
	if err == nil {
		t.Fatal("expected error when mkdir fails")
	}
	expectedPath := filepath.FromSlash("/tmp/usercfg/ttscli")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to include config dir path, got: %v", err)
	}
}

func TestSaveConfigPathError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	userConfigDir = func() (string, error) {
		return "", errors.New("config dir failed")
	}

	err := SaveConfig(Config{Profiles: map[string]Profile{}})
	if err == nil || !strings.Contains(err.Error(), "resolve config path") {
		t.Fatalf("expected resolve config path error, got: %v", err)
	}
}

func TestSaveConfigWriteError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	writeFile = func(path string, data []byte, perm os.FileMode) error {
		return errors.New("write failed")
	}

	err := SaveConfig(Config{Profiles: map[string]Profile{}})
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	expectedPath := filepath.FromSlash("/tmp/usercfg/ttscli/config.json.tmp")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to include tmp file path, got: %v", err)
	}
}

func TestSaveConfigRenameErrorCleansUpTmp(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	var writtenPath string
	var removed string
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		writtenPath = path
		return nil
	}
	renameFile = func(oldpath, newpath string) error {
		return errors.New("rename failed")
	}
	removeFile = func(path string) error {
		removed = path
		return nil
	}

	err := SaveConfig(Config{Profiles: map[string]Profile{}})
	if err == nil {
		t.Fatal("expected error when rename fails")
	}
	if !strings.Contains(err.Error(), "finalize config file") {
		t.Errorf("expected finalize error wrapping, got: %v", err)
	}
	expectedPath := filepath.FromSlash("/tmp/usercfg/ttscli/config.json")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to include config path, got: %v", err)
	}
	if removed == "" || removed != writtenPath {
		t.Errorf("expected tmp file %q to be removed, got %q", writtenPath, removed)
	}
}

// GetProfile tests

func TestGetProfileFound(t *testing.T) {
	cfg := Config{
		Profiles: map[string]Profile{
			"gcp:default": {Provider: "gcp", Name: "default"},
		},
	}
	profile, err := GetProfile(cfg, "gcp:default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Provider != "gcp" || profile.Name != "default" {
		t.Errorf("unexpected profile: %+v", profile)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	cfg := Config{Profiles: map[string]Profile{}}
	_, err := GetProfile(cfg, "gcp:nonexistent")
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}
