package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

func runSetupCommand(stdout, stderr io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	reader := bufio.NewReader(setupInput)

	fmt.Fprintln(stdout, "Welcome to ttscli setup.")
	fmt.Fprintln(stdout, "This will create a new GCP profile for text-to-speech.")
	fmt.Fprintln(stdout)

	profileName := "default"
	profileKey := "gcp:" + profileName

	if len(appCfg.Profiles) > 0 {
		inputProfileName, err := promptLine(reader, stdout, "Profile name [default]: ")
		if err != nil {
			return fmt.Errorf("read profile name: %w", err)
		}
		profileName = strings.TrimSpace(inputProfileName)
		if profileName == "" {
			profileName = "default"
		}

		profileKey = "gcp:" + profileName
		if _, exists := appCfg.Profiles[profileKey]; exists {
			fmt.Fprintf(stdout, "Profile %s already exists. Use 'ttscli profile use %s' to activate it.\n", profileKey, profileKey)
			return nil
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Press Enter on language/voice to use built-in defaults: %s / %s.\n", cli.DefaultLanguage, cli.DefaultVoice)

	inputAPIKey, err := promptLine(reader, stdout, "Google Cloud API key: ")
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	apiKey := strings.TrimSpace(inputAPIKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}

	inputLang, err := promptLine(reader, stdout, fmt.Sprintf("Default language [%s] (press Enter to use default): ", cli.DefaultLanguage))
	if err != nil {
		return fmt.Errorf("read language: %w", err)
	}
	lang := strings.TrimSpace(inputLang)
	if lang == "" {
		lang = cli.DefaultLanguage
	}

	inputVoice, err := promptLine(reader, stdout, fmt.Sprintf("Default voice [%s] (press Enter to use default): ", cli.DefaultVoice))
	if err != nil {
		return fmt.Errorf("read voice: %w", err)
	}
	voice := strings.TrimSpace(inputVoice)
	if voice == "" {
		voice = cli.DefaultVoice
	}

	fmt.Fprintln(stdout, "Validating API key and voice...")

	profile := config.Profile{
		Provider: "gcp",
		Name:     profileName,
		Credentials: map[string]interface{}{
			"apiKey": apiKey,
		},
		Defaults: map[string]string{
			"lang":  lang,
			"voice": voice,
		},
	}

	provider, err := newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	ctx, stop := newAppCtx()
	defer stop()

	voices, err := provider.ListVoices(ctx, lang)
	if err != nil {
		return fmt.Errorf("validate provider: %w", err)
	}
	if !voiceExists(voice, voices) {
		return fmt.Errorf("voice %q is not available for language %q", voice, lang)
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = "gcp"
		appCfg.ActiveProfile = profileName
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	runCheck, err := promptYesNo(reader, stdout, "Run sound check now? [Y/n]: ", true)
	if err != nil {
		return fmt.Errorf("read sound check choice: %w", err)
	}
	if runCheck {
		fmt.Fprintln(stdout, "Running sound check...")
		req := tts.SynthRequest{
			Text:          setupSoundCheckText,
			LanguageCode:  lang,
			VoiceName:     voice,
			AudioEncoding: tts.AudioEncodingMP3,
		}
		audioBytes, err := provider.SynthesizeRequest(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to synthesize sound check: %w", err)
		}
		fmt.Fprintln(stdout, "Playing audio...")
		if err := playAudio(audioBytes, stdout, stderr); err != nil {
			return fmt.Errorf("failed to play sound check: %w", err)
		}
	}

	cfgPath, err := config.Path()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Setup complete.")
	fmt.Fprintf(stdout, "Created profile: %s\n", profileKey)
	fmt.Fprintf(stdout, "Profile settings: voice=%s lang=%s apiKey=%s\n", voice, lang, maskAPIKey(apiKey))
	if appCfg.ActiveProvider == "gcp" && appCfg.ActiveProfile == profileName {
		fmt.Fprintf(stdout, "Profile set as active.\n")
	}
	fmt.Fprintf(stdout, "Config file: %s\n", cfgPath)
	return nil
}
