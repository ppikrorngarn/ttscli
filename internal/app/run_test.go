package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

type fakeTTSClient struct {
	listVoicesFn func(ctx context.Context, langCode string) ([]tts.Voice, error)
	synthesizeFn func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error)
}

func (f *fakeTTSClient) ListVoices(ctx context.Context, langCode string) ([]tts.Voice, error) {
	return f.listVoicesFn(ctx, langCode)
}

func (f *fakeTTSClient) Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
	return f.synthesizeFn(ctx, text, languageCode, voiceName, audioEncoding)
}

func TestRunMissingAPIKeyEnv(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "" }

	err := Run([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing env var error")
	}
	if !strings.Contains(err.Error(), apiKeyEnvVar) {
		t.Fatalf("expected env var in error, got %v", err)
	}
}

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

func TestRunListVoicesSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{ListVoices: true, Lang: "en-GB"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }

	printCalled := false
	printVoices = func(w io.Writer, langCode string, voices []tts.Voice) {
		printCalled = true
	}

	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return []tts.Voice{{Name: "en-GB-Neural2-B"}}, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				t.Fatal("Synthesize should not be called in list-voices mode")
				return nil, nil
			},
		}
	}

	if err := Run([]string{"--list-voices"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !printCalled {
		t.Fatal("expected printVoices to be called")
	}
}

func TestRunListVoicesError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{ListVoices: true, Lang: "en-GB"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				return nil, errors.New("list failure")
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, nil
			},
		}
	}

	err := Run([]string{"--list-voices"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "failed to list voices") {
		t.Fatalf("expected list voices error, got: %v", err)
	}
}

func TestRunSynthesizeError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true, Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return nil, errors.New("synth failure")
			},
		}
	}

	err := Run([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "failed to synthesize") {
		t.Fatalf("expected synthesize error, got: %v", err)
	}
}

func TestRunSaveError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", SavePath: "out.mp3", Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}
	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("write failure")
	}

	err := Run([]string{"--text", "hello", "--save", "out.mp3"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "failed to save file") {
		t.Fatalf("expected save error, got: %v", err)
	}
}

func TestRunPlayError(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true, Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error {
		return errors.New("play failure")
	}

	err := Run([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "failed to play audio") {
		t.Fatalf("expected play error, got: %v", err)
	}
}

func TestRunSaveSuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", SavePath: "out.mp3", Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}

	writeCalled := false
	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		writeCalled = true
		if name != "out.mp3" {
			t.Fatalf("unexpected save path: %s", name)
		}
		if string(data) != "audio" {
			t.Fatalf("unexpected audio payload: %s", string(data))
		}
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"--text", "hello", "--save", "out.mp3"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !writeCalled {
		t.Fatal("expected writeFile to be called")
	}
	out := stdout.String()
	if !strings.Contains(out, "Synthesizing speech...") || !strings.Contains(out, "Saved audio to: out.mp3") {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func TestRunPlaySuccess(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true, Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}

	playCalled := false
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error {
		playCalled = true
		if string(audioBytes) != "audio" {
			t.Fatalf("unexpected audio payload: %s", string(audioBytes))
		}
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"--text", "hello", "--play"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !playCalled {
		t.Fatal("expected playAudio to be called")
	}
	out := stdout.String()
	if !strings.Contains(out, "Synthesizing speech...") || !strings.Contains(out, "Playing audio...") {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func TestRunUsesAppContextForSynthesize(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true, Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	newAppCtx = func() (context.Context, context.CancelFunc) {
		return ctx, func() {}
	}

	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				if !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatalf("expected canceled context, got: %v", ctx.Err())
				}
				return nil, context.Canceled
			},
		}
	}

	err := Run([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "failed to synthesize") {
		t.Fatalf("expected synthesize error on canceled context, got: %v", err)
	}
}

func TestRunCallsContextStop(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	parseArgs = func(args []string, stderr io.Writer) (cli.Config, error) {
		return cli.Config{Text: "hello", Play: true, Lang: "en-US", Voice: "en-US-Neural2-F"}, nil
	}
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }

	stopCalled := false
	newAppCtx = func() (context.Context, context.CancelFunc) {
		return context.Background(), func() { stopCalled = true }
	}

	newTTSClient = func(apiKey string) ttsService {
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) { return nil, nil },
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				return []byte("audio"), nil
			},
		}
	}
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error { return nil }

	if err := Run([]string{"--text", "hello", "--play"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !stopCalled {
		t.Fatal("expected context stop function to be called")
	}
}

func TestRunEndToEndFlowWithRealParseArgs(t *testing.T) {
	reset := stubAppDeps()
	defer reset()

	// Use real CLI parsing to validate package wiring.
	parseArgs = cli.ParseArgs
	loadDotenv = func(...string) error { return nil }
	lookupEnv = func(_ string) string { return "k" }
	newAppCtx = func() (context.Context, context.CancelFunc) {
		return context.Background(), func() {}
	}

	var gotText, gotLang, gotVoice, gotEncoding string
	newTTSClient = func(apiKey string) ttsService {
		if apiKey != "k" {
			t.Fatalf("unexpected api key: %q", apiKey)
		}
		return &fakeTTSClient{
			listVoicesFn: func(ctx context.Context, langCode string) ([]tts.Voice, error) {
				t.Fatal("ListVoices should not be called")
				return nil, nil
			},
			synthesizeFn: func(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
				gotText = text
				gotLang = languageCode
				gotVoice = voiceName
				gotEncoding = audioEncoding
				return []byte("audio"), nil
			},
		}
	}

	saved := false
	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		saved = true
		if name != "out.mp3" {
			t.Fatalf("unexpected save path: %q", name)
		}
		if string(data) != "audio" {
			t.Fatalf("unexpected save payload: %q", string(data))
		}
		return nil
	}

	played := false
	playAudio = func(audioBytes []byte, stdout, stderr io.Writer) error {
		played = true
		if string(audioBytes) != "audio" {
			t.Fatalf("unexpected play payload: %q", string(audioBytes))
		}
		return nil
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"--text", "hello world",
		"--lang", "en-GB",
		"--voice", "en-GB-Neural2-B",
		"--save", "out.mp3",
		"--play",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if gotText != "hello world" || gotLang != "en-GB" || gotVoice != "en-GB-Neural2-B" {
		t.Fatalf("unexpected synth inputs: text=%q lang=%q voice=%q", gotText, gotLang, gotVoice)
	}
	if gotEncoding != tts.AudioEncodingMP3 {
		t.Fatalf("unexpected encoding: %q", gotEncoding)
	}
	if !saved || !played {
		t.Fatalf("expected saved and played, got saved=%v played=%v", saved, played)
	}

	out := stdout.String()
	if !strings.Contains(out, "Synthesizing speech...") ||
		!strings.Contains(out, "Saved audio to: out.mp3") ||
		!strings.Contains(out, "Playing audio...") {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func stubAppDeps() func() {
	oldParseArgs := parseArgs
	oldLoadDotenv := loadDotenv
	oldLookupEnv := lookupEnv
	oldNewTTSClient := newTTSClient
	oldPrintVoices := printVoices
	oldWriteFile := writeFile
	oldPlayAudio := playAudio
	oldNewAppCtx := newAppCtx

	return func() {
		parseArgs = oldParseArgs
		loadDotenv = oldLoadDotenv
		lookupEnv = oldLookupEnv
		newTTSClient = oldNewTTSClient
		printVoices = oldPrintVoices
		writeFile = oldWriteFile
		playAudio = oldPlayAudio
		newAppCtx = oldNewAppCtx
	}
}
