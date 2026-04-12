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
	removeFile     = os.Remove
	currentGOOS    = func() string { return runtime.GOOS }
	lookPathCmd    = exec.LookPath
	playCommand    = buildPlayCommand
)

func PlayAudio(audioBytes []byte, stdout, stderr io.Writer) error {
	tmpFile, err := createTempFile("", tempAudioPattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpFilePath := tmpFile.Name()
	defer removeFile(tmpFilePath)

	if _, err := tmpFile.Write(audioBytes); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	cmd, err := playCommand(currentGOOS(), tmpFilePath, lookPathCmd)
	if err != nil {
		return err
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
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
