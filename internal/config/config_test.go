package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func stubConfigDeps() func() {
	oldUserConfigDir := userConfigDir
	oldExecutablePath := executablePath
	oldFileExists := fileExists
	oldReadFile := readFile
	oldMkdirAll := mkdirAll
	oldRemoveFile := removeFile

	userConfigDir = func() (string, error) {
		return "/tmp/usercfg", nil
	}
	executablePath = func() (string, error) {
		return "/tmp/app/config.json", nil
	}
	fileExists = func(path string) (bool, error) {
		if path == "/tmp/app/config.json" {
			return false, nil
		}
		return false, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
	}
	readFile = func(path string) ([]byte, error) {
		return nil, &fs.PathError{Op: "read", Path: path, Err: fs.ErrNotExist}
	}
	mkdirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	removeFile = func(path string) error {
		return nil
	}

	return func() {
		userConfigDir = oldUserConfigDir
		executablePath = oldExecutablePath
		fileExists = oldFileExists
		readFile = oldReadFile
		mkdirAll = oldMkdirAll
		removeFile = oldRemoveFile
	}
}

func TestPath(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	got, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	// Since the local config doesn't exist, it falls back to user path
	want := filepath.Join("/tmp/usercfg", appDirName, configName)
	if got != want {
		t.Fatalf("unexpected path: got=%q want=%q", got, want)
	}
}

func TestPathPrefersLocalConfigNextToBinaryWhenExists(t *testing.T) {
	reset := stubConfigDeps()
	defer reset()

	fileExists = func(path string) (bool, error) {
		return path == "/tmp/app/config.json", nil
	}

	got, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	want := "/tmp/app/config.json"
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
	want := filepath.Join("/tmp/usercfg", appDirName, configName)
	if got != want {
		t.Fatalf("unexpected path: got=%q want=%q", got, want)
	}
}
