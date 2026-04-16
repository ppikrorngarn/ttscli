package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestRunDoctorSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeDoctor}, nil
	}
	lookupEnv = func(_ string) string { return "env-key" }
	currentGOOS = func() string { return "linux" }
	lookPathCmd = func(file string) (string, error) {
		if file == "mpg123" {
			return "/usr/bin/mpg123", nil
		}
		return "", errors.New("not found")
	}
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Doctor result: OK") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunDoctorFailsWhenNoAPIKey(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeDoctor}, nil
	}
	lookupEnv = func(_ string) string { return "" }
	loadDefaults = func() (config.Defaults, error) {
		return config.Defaults{}, nil
	}
	currentGOOS = func() string { return "linux" }
	lookPathCmd = func(file string) (string, error) {
		if file == "mpg123" {
			return "/usr/bin/mpg123", nil
		}
		return "", errors.New("not found")
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor"}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected doctor failure when api key is missing")
	}
	out := stdout.String()
	if !strings.Contains(out, "[FAIL] API key availability") ||
		!strings.Contains(out, "Doctor result: FAIL") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunDoctorFailsWhenPlaybackUnavailable(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeDoctor}, nil
	}
	lookupEnv = func(_ string) string { return "env-key" }
	currentGOOS = func() string { return "linux" }
	lookPathCmd = func(file string) (string, error) { return "", errors.New("not found") }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-US-Neural2-F"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor"}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected doctor failure when playback is unavailable")
	}
	out := stdout.String()
	if !strings.Contains(out, "[FAIL] Audio playback") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunDoctorFailsWhenConfigUnreadable(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Mode: cli.ModeDoctor}, nil
	}
	loadDefaults = func() (config.Defaults, error) { return config.Defaults{}, errors.New("bad config") }
	loadConfig = func() (config.Config, error) { return config.Config{}, errors.New("bad config") }
	lookupEnv = func(_ string) string { return "" }
	currentGOOS = func() string { return "linux" }
	lookPathCmd = func(file string) (string, error) {
		if file == "mpg123" {
			return "/usr/bin/mpg123", nil
		}
		return "", errors.New("not found")
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor"}, &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected doctor failure when config is unreadable")
	}
	out := stdout.String()
	if !strings.Contains(out, "[FAIL] Config file") {
		t.Fatalf("unexpected output: %q", out)
	}
}
