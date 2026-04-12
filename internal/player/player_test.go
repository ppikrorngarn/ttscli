package player

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildPlayCommandDarwin(t *testing.T) {
	cmd, err := buildPlayCommand("darwin", "/tmp/a.mp3", func(string) (string, error) {
		return "", errors.New("not used")
	})
	if err != nil {
		t.Fatalf("buildPlayCommand returned error: %v", err)
	}
	if filepath.Base(cmd.Path) != "afplay" {
		t.Fatalf("expected afplay, got %q", cmd.Path)
	}
}

func TestBuildPlayCommandLinuxFallbackOrder(t *testing.T) {
	lookPath := func(file string) (string, error) {
		if file == "ffplay" {
			return "/usr/bin/ffplay", nil
		}
		return "", errors.New("missing")
	}
	cmd, err := buildPlayCommand("linux", "/tmp/a.mp3", lookPath)
	if err != nil {
		t.Fatalf("buildPlayCommand returned error: %v", err)
	}
	if filepath.Base(cmd.Path) != "ffplay" {
		t.Fatalf("expected ffplay, got %q", cmd.Path)
	}
}

func TestBuildPlayCommandLinuxNoPlayers(t *testing.T) {
	_, err := buildPlayCommand("linux", "/tmp/a.mp3", func(string) (string, error) {
		return "", errors.New("missing")
	})
	if err == nil {
		t.Fatal("expected error when no players are available")
	}
}

func TestBuildPlayCommandUnsupportedOS(t *testing.T) {
	_, err := buildPlayCommand("plan9", "/tmp/a.mp3", func(string) (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
}

func TestPlayAudioCreateTempError(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	createTempFile = func(dir, pattern string) (*os.File, error) {
		return nil, errors.New("create temp failed")
	}

	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected create temp file error")
	}
}

func TestPlayAudioBuildCommandError(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	playCommand = func(goos, filePath string, lookPath func(file string) (string, error)) (*exec.Cmd, error) {
		return nil, errors.New("build command failed")
	}

	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected build command error")
	}
}

func stubPlayerDeps() func() {
	oldCreateTempFile := createTempFile
	oldRemoveFile := removeFile
	oldCurrentGOOS := currentGOOS
	oldLookPathCmd := lookPathCmd
	oldPlayCommand := playCommand

	return func() {
		createTempFile = oldCreateTempFile
		removeFile = oldRemoveFile
		currentGOOS = oldCurrentGOOS
		lookPathCmd = oldLookPathCmd
		playCommand = oldPlayCommand
	}
}
