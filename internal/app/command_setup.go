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

	fmt.Fprintln(stdout, "Welcome to ttscli setup!")
	fmt.Fprintln(stdout, "This guided setup will create a Google Cloud profile for text-to-speech.")
	fmt.Fprintln(stdout)

	profileName := "default"
	profileKey := "gcp:" + profileName

	if len(appCfg.Profiles) > 0 {
		fmt.Fprintln(stdout, "You already have profiles configured.")
		inputProfileName, err := promptLine(reader, stdout, "Enter a name for the new profile [default]: ")
		if err != nil {
			return fmt.Errorf("read profile name: %w", err)
		}
		profileName = strings.TrimSpace(inputProfileName)
		if profileName == "" {
			profileName = "default"
		}

		profileKey = "gcp:" + profileName
		if _, exists := appCfg.Profiles[profileKey]; exists {
			fmt.Fprintf(stdout, "Profile '%s' already exists.\n", profileKey)
			fmt.Fprintf(stdout, "Tip: Use 'ttscli profile use %s' to activate it.\n", profileKey)
			return nil
		}
	}

	fmt.Fprintf(stdout, "Note: Press Enter to use the built-in defaults (%s / %s).\n", cli.DefaultLanguage, cli.DefaultVoice)
	fmt.Fprintln(stdout)

	inputAPIKey, err := promptLine(reader, stdout, "Enter your Google Cloud API key: ")
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	apiKey := strings.TrimSpace(inputAPIKey)
	if apiKey == "" {
		return fmt.Errorf("API key is required. You can create one in Google Cloud Console under APIs & Services > Credentials")
	}

	inputLang, err := promptLine(reader, stdout, fmt.Sprintf("Default language code [%s]: ", cli.DefaultLanguage))
	if err != nil {
		return fmt.Errorf("read language: %w", err)
	}
	lang := strings.TrimSpace(inputLang)
	if lang == "" {
		lang = cli.DefaultLanguage
	}

	inputVoice, err := promptLine(reader, stdout, fmt.Sprintf("Default voice name [%s]: ", cli.DefaultVoice))
	if err != nil {
		return fmt.Errorf("read voice: %w", err)
	}
	voice := strings.TrimSpace(inputVoice)
	if voice == "" {
		voice = cli.DefaultVoice
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Validating API key and voice configuration...")

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
		return fmt.Errorf("voice '%s' is not available for language '%s'. Run 'ttscli voices --lang %s' to see available voices", voice, lang, lang)
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = "gcp"
		appCfg.ActiveProfile = profileName
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintln(stdout)
	runCheck, err := promptYesNo(reader, stdout, "Would you like to run a sound check now? [Y/n]: ", true)
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
		fmt.Fprintln(stdout, "Sound check completed!")
	}

	cfgPath, err := config.Path()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "✓ Setup complete!")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Created profile: %s\n", profileKey)
	fmt.Fprintf(stdout, "  Voice:     %s\n", voice)
	fmt.Fprintf(stdout, "  Language:  %s\n", lang)
	fmt.Fprintf(stdout, "  API Key:   %s\n", maskAPIKey(apiKey))
	if appCfg.ActiveProvider == "gcp" && appCfg.ActiveProfile == profileName {
		fmt.Fprintln(stdout, "  Status:    Active")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Config file: %s\n", cfgPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  • Run 'ttscli speak --text \"Hello world\"' to test speech synthesis")
	fmt.Fprintln(stdout, "  • Run 'ttscli voices' to list available voices")
	fmt.Fprintln(stdout, "  • Run 'ttscli doctor' to verify your configuration")
	return nil
}
