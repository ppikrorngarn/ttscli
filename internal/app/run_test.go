package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func TestRunParseArgsError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{}, errors.New("parse failed")
	}

	err := Run([]string{"--bad-flag"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "parse failed") {
		t.Fatalf("expected parse error passthrough, got: %v", err)
	}
}

func TestRunEndToEndFlowWithRealParseArgs(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	listCalled := false
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				listCalled = true
				return []tts.Voice{{Name: "en-GB-Neural2-B"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}, nil
	}

	err := Run([]string{"voices", "--lang", "en-GB"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !listCalled {
		t.Fatal("expected provider ListVoices to be called")
	}
}

type fakeTTSClient struct {
	listVoicesFn func(ctx context.Context, langCode string) ([]tts.Voice, error)
	synthesizeFn func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

func (f *fakeTTSClient) Name() string {
	return "gcp"
}

func (f *fakeTTSClient) ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error) {
	return f.listVoicesFn(ctx, langCode)
}

func (f *fakeTTSClient) Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
	return f.synthesizeFn(ctx, text, languageCode, voiceName, audioEncoding)
}

func (f *fakeTTSClient) SynthesizeRequest(ctx context.Context, req tts.SynthRequest) ([]byte, error) {
	return f.synthesizeFn(ctx, req.Text, req.LanguageCode, req.VoiceName, req.AudioEncoding)
}

func (f *fakeTTSClient) DefaultVoice(langCode string) string {
	if langCode == "en-US" {
		return "en-US-Neural2-F"
	}
	return ""
}

func stubAppDeps() func() {
	oldParseArgs := parseArgs
	oldLookupEnv := lookupEnv
	oldCurrentGOOS := currentGOOS
	oldLookPathCmd := lookPathCmd
	oldNewProvider := newProvider
	oldLoadConfig := loadConfig
	oldSaveConfig := saveConfig
	oldGetProfile := getProfile
	oldPrintVoices := printVoices
	oldWriteFile := writeFile
	oldPlayAudio := playAudio
	oldSetupInput := setupInput
	oldNewAppCtx := newAppCtx

	loadConfig = func() (config.Config, error) {
		return config.Config{
			ActiveProvider: "gcp",
			ActiveProfile:  "default",
			Profiles: map[string]config.Profile{
				"gcp:default": {
					Provider: "gcp",
					Name:     "default",
					Credentials: map[string]interface{}{
						"apiKey": "test-api-key",
					},
					Defaults: map[string]string{
						"lang":  "en-US",
						"voice": "en-US-Neural2-F",
					},
				},
			},
		}, nil
	}
	newProvider = func(profile config.Profile) (tts.Provider, error) {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-GB-Neural2-B"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}, nil
	}
	saveConfig = func(config.Config) error { return nil }
	getProfile = func(cfg config.Config, key string) (config.Profile, error) {
		if profile, ok := cfg.Profiles[key]; ok {
			return profile, nil
		}
		return config.Profile{}, fmt.Errorf("profile not found")
	}

	return func() {
		parseArgs = oldParseArgs
		lookupEnv = oldLookupEnv
		currentGOOS = oldCurrentGOOS
		lookPathCmd = oldLookPathCmd
		newProvider = oldNewProvider
		loadConfig = oldLoadConfig
		saveConfig = oldSaveConfig
		getProfile = oldGetProfile
		printVoices = oldPrintVoices
		writeFile = oldWriteFile
		playAudio = oldPlayAudio
		setupInput = oldSetupInput
		newAppCtx = oldNewAppCtx
	}
}
