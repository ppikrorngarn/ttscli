package player

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

const tempAudioPattern = "ttscli-*.mp3"

var (
	createTempFile = os.CreateTemp
	writeTempFile  = func(f *os.File, audioBytes []byte) (int, error) { return f.Write(audioBytes) }
	closeTempFile  = func(f *os.File) error { return f.Close() }
	removeFile     = os.Remove
	currentGOOS    = func() string { return runtime.GOOS }
	lookPathCmd    = exec.LookPath
	playCommand    = buildPlayCommand
	runCommand     = func(cmd *exec.Cmd) error { return cmd.Run() }
)

func PlayAudio(audioBytes []byte, stdout, stderr io.Writer) error {
	tmpFile, err := createTempFile("", tempAudioPattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpFilePath := tmpFile.Name()
	defer func() {
		if err := removeFile(tmpFilePath); err != nil {
			fmt.Fprintf(stderr, "warning: failed to remove temp file %s: %v\n", tmpFilePath, err)
		}
	}()

	if _, err := writeTempFile(tmpFile, audioBytes); err != nil {
		_ = closeTempFile(tmpFile)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := closeTempFile(tmpFile); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	cmd, err := playCommand(currentGOOS(), tmpFilePath, lookPathCmd)
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := runCommand(cmd); err != nil {
		return fmt.Errorf("run player command: %w", err)
	}
	return nil
}

func buildPlayCommand(goos, filePath string, lookPath func(file string) (string, error)) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("afplay", filePath), nil
	case "linux":
		if _, err := lookPath("mpg123"); err == nil {
			return exec.Command("mpg123", "-q", filePath), nil
		}
		if _, err := lookPath("paplay"); err == nil {
			return exec.Command("paplay", filePath), nil
		}
		if _, err := lookPath("ffplay"); err == nil {
			return exec.Command("ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", filePath), nil
		}
		return nil, errors.New("no supported audio player found on Linux (try installing mpg123)")
	case "windows":
		psScript := fmt.Sprintf(`(New-Object Media.SoundPlayer "%s").PlaySync()`, filePath)
		return exec.Command("powershell", "-c", psScript), nil
	default:
		return nil, fmt.Errorf("unsupported platform for audio playback: %s", goos)
	}
}
