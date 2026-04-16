package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
)

func TestRunSpeakMode(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSpeak, Text: "hello", Lang: "en-US", Voice: "en-US-Neural2-F", Play: true}, nil
	}
	playCalled := false
	playAudio = func(audio []byte, stdout, stderr io.Writer) error {
		playCalled = true
		return nil
	}

	if err := Run([]string{"speak", "--text", "hello"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !playCalled {
		t.Error("expected playAudio to be called for speak mode")
	}
}

func TestRunSaveMode(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeSave, Text: "hello", Lang: "en-US", Voice: "en-US-Neural2-F", SavePath: "out.mp3"}, nil
	}
	var savedPath string
	writeFile = func(path string, data []byte, _ os.FileMode) error {
		savedPath = path
		return nil
	}

	if err := Run([]string{"save", "--text", "hello", "--out", "out.mp3"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if savedPath != "out.mp3" {
		t.Errorf("expected file saved to out.mp3, got %q", savedPath)
	}
}

func TestRunProfileFlag(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeVoices, ListVoices: true, Lang: "en-US", Profile: "gcp:default"}, nil
	}

	if err := Run([]string{"voices", "--profile", "gcp:default"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunEnvVarProfile(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeVoices, ListVoices: true, Lang: "en-US"}, nil
	}
	lookupEnv = func(key string) string {
		if key == "TTSCLI_PROFILE" {
			return "gcp:default"
		}
		return ""
	}

	if err := Run([]string{"voices"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunNoActiveProfile(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeVoices, ListVoices: true, Lang: "en-US"}, nil
	}
	loadConfig = func() (config.Config, error) {
		return config.Config{
			Profiles: map[string]config.Profile{
				"gcp:default": {Provider: "gcp", Name: "default"},
			},
		}, nil
	}

	err := Run([]string{"voices"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no active profile") {
		t.Fatalf("expected no active profile error, got: %v", err)
	}
}
