package player

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestBuildPlayCommandLinuxPrefersMpg123(t *testing.T) {
	lookPath := func(file string) (string, error) {
		if file == "mpg123" {
			return "/usr/bin/mpg123", nil
		}
		if file == "paplay" || file == "ffplay" {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("missing")
	}
	cmd, err := buildPlayCommand("linux", "/tmp/a.mp3", lookPath)
	if err != nil {
		t.Fatalf("buildPlayCommand returned error: %v", err)
	}
	if filepath.Base(cmd.Path) != "mpg123" {
		t.Fatalf("expected mpg123, got %q", cmd.Path)
	}
}

func TestBuildPlayCommandLinuxFallsBackToPaplay(t *testing.T) {
	lookPath := func(file string) (string, error) {
		if file == "paplay" {
			return "/usr/bin/paplay", nil
		}
		return "", errors.New("missing")
	}
	cmd, err := buildPlayCommand("linux", "/tmp/a.mp3", lookPath)
	if err != nil {
		t.Fatalf("buildPlayCommand returned error: %v", err)
	}
	if filepath.Base(cmd.Path) != "paplay" {
		t.Fatalf("expected paplay, got %q", cmd.Path)
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

func TestBuildPlayCommandWindowsWithSpaces(t *testing.T) {
	cmd, err := buildPlayCommand("windows", `C:\Temp Dir\a b.mp3`, func(string) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatalf("buildPlayCommand returned error: %v", err)
	}
	if filepath.Base(cmd.Path) != "powershell" {
		t.Fatalf("expected powershell, got %q", cmd.Path)
	}
	if len(cmd.Args) < 3 {
		t.Fatalf("unexpected powershell args: %#v", cmd.Args)
	}
	if got := cmd.Args[2]; !bytes.Contains([]byte(got), []byte(`C:\Temp Dir\a b.mp3`)) {
		t.Fatalf("expected script to include file path, got %q", got)
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

func TestPlayAudioWriteTempError(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	writeTempFile = func(_ *os.File, _ []byte) (int, error) {
		return 0, errors.New("write failed")
	}

	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "write temp file") {
		t.Fatalf("expected write temp file error, got: %v", err)
	}
}

func TestPlayAudioCloseTempError(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	closeCallCount := 0
	closeTempFile = func(_ *os.File) error {
		closeCallCount++
		if closeCallCount == 1 {
			return errors.New("close failed")
		}
		return nil
	}

	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "close temp file") {
		t.Fatalf("expected close temp file error, got: %v", err)
	}
}

func TestPlayAudioRunCommandError(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	playCommand = func(goos, filePath string, lookPath func(file string) (string, error)) (*exec.Cmd, error) {
		return exec.Command("echo"), nil
	}
	runCommand = func(_ *exec.Cmd) error {
		return errors.New("run failed")
	}

	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "run player command") {
		t.Fatalf("expected run player command error, got: %v", err)
	}
}

func TestPlayAudioRemoveTempWarning(t *testing.T) {
	reset := stubPlayerDeps()
	defer reset()

	playCommand = func(goos, filePath string, lookPath func(file string) (string, error)) (*exec.Cmd, error) {
		return exec.Command("echo"), nil
	}
	runCommand = func(_ *exec.Cmd) error { return nil }
	removeFile = func(_ string) error {
		return errors.New("remove failed")
	}

	var stderr bytes.Buffer
	err := PlayAudio([]byte("audio"), &bytes.Buffer{}, &stderr)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning: failed to remove temp file") {
		t.Fatalf("expected cleanup warning in stderr, got: %q", stderr.String())
	}
}

func stubPlayerDeps() func() {
	oldCreateTempFile := createTempFile
	oldWriteTempFile := writeTempFile
	oldCloseTempFile := closeTempFile
	oldRemoveFile := removeFile
	oldCurrentGOOS := currentGOOS
	oldLookPathCmd := lookPathCmd
	oldPlayCommand := playCommand
	oldRunCommand := runCommand

	return func() {
		createTempFile = oldCreateTempFile
		writeTempFile = oldWriteTempFile
		closeTempFile = oldCloseTempFile
		removeFile = oldRemoveFile
		currentGOOS = oldCurrentGOOS
		lookPathCmd = oldLookPathCmd
		playCommand = oldPlayCommand
		runCommand = oldRunCommand
	}
}
