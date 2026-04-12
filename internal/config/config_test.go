package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	userConfigDir = func() (string, error) {
		return "/tmp/usercfg", nil
	}

	got, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	want := filepath.Join("/tmp/usercfg", appDirName, configName)
	if got != want {
		t.Fatalf("unexpected path: got=%q want=%q", got, want)
	}
}

func TestLoadDefaultsNotExists(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	defaults, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults returned error: %v", err)
	}
	if defaults.Voice != "" || defaults.Lang != "" {
		t.Fatalf("expected empty defaults, got %+v", defaults)
	}
}

func TestLoadDefaultsInvalidJSON(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	readFile = func(string) ([]byte, error) {
		return []byte("{invalid"), nil
	}

	_, err := LoadDefaults()
	if err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestSaveDefaultsWritesFile(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	mkdirCalled := false
	mkdirAll = func(path string, perm os.FileMode) error {
		mkdirCalled = true
		return nil
	}

	wrote := false
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		wrote = true
		if !strings.Contains(string(data), `"voice": "v1"`) || !strings.Contains(string(data), `"lang": "en-GB"`) {
			t.Fatalf("unexpected config payload: %q", string(data))
		}
		return nil
	}

	err := SaveDefaults(Defaults{Voice: "v1", Lang: "en-GB"})
	if err != nil {
		t.Fatalf("SaveDefaults returned error: %v", err)
	}
	if !mkdirCalled || !wrote {
		t.Fatalf("expected mkdir and write to be called, mkdir=%v wrote=%v", mkdirCalled, wrote)
	}
}

func TestSaveDefaultsMkdirError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	mkdirAll = func(path string, perm os.FileMode) error {
		return errors.New("mkdir failed")
	}

	err := SaveDefaults(Defaults{Voice: "v1"})
	if err == nil || !strings.Contains(err.Error(), "create config dir") {
		t.Fatalf("expected mkdir error, got: %v", err)
	}
}

func TestClearDefaults(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	err := ClearDefaults()
	if err != nil {
		t.Fatalf("ClearDefaults returned error: %v", err)
	}
}

func TestClearDefaultsError(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	removeFile = func(string) error {
		return errors.New("remove failed")
	}

	err := ClearDefaults()
	if err == nil || !strings.Contains(err.Error(), "remove config file") {
		t.Fatalf("expected remove error, got: %v", err)
	}
}

func stubConfigDeps() func() {
	oldUserConfigDir := userConfigDir
	oldReadFile := readFile
	oldWriteFile := writeFile
	oldMkdirAll := mkdirAll
	oldRemoveFile := removeFile

	userConfigDir = func() (string, error) {
		return "/tmp/usercfg", nil
	}
	readFile = func(string) ([]byte, error) {
		return []byte(`{"voice":"en-US-Neural2-F","lang":"en-US"}`), nil
	}
	writeFile = func(string, []byte, os.FileMode) error { return nil }
	mkdirAll = func(string, os.FileMode) error { return nil }
	removeFile = func(string) error { return nil }

	return func() {
		userConfigDir = oldUserConfigDir
		readFile = oldReadFile
		writeFile = oldWriteFile
		mkdirAll = oldMkdirAll
		removeFile = oldRemoveFile
	}
}
