package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

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

	spec, err := chooseSetupProvider(reader, stdout)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "This guided setup will create a %s profile for text-to-speech.\n", spec.DisplayName)
	fmt.Fprintln(stdout)

	profileName := "default"
	profileKey := spec.Name + ":" + profileName

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

		var buildErr error
		profileKey, _, profileName, buildErr = config.BuildProfileKey(spec.Name, profileName)
		if buildErr != nil {
			return buildErr
		}
		if _, exists := appCfg.Profiles[profileKey]; exists {
			fmt.Fprintf(stdout, "Profile '%s' already exists.\n", profileKey)
			fmt.Fprintf(stdout, "Tip: Use 'ttscli profile use %s' to activate it.\n", profileKey)
			return nil
		}
	}

	fmt.Fprintf(stdout, "Note: Press Enter to use the built-in defaults (%s / %s).\n", spec.DefaultLang, spec.DefaultVoice)
	fmt.Fprintln(stdout)

	creds, err := promptCredentials(reader, stdout, spec)
	if err != nil {
		return err
	}

	inputLang, err := promptLine(reader, stdout, fmt.Sprintf("Default language code [%s]: ", spec.DefaultLang))
	if err != nil {
		return fmt.Errorf("read language: %w", err)
	}
	lang := strings.TrimSpace(inputLang)
	if lang == "" {
		lang = spec.DefaultLang
	}

	inputVoice, err := promptLine(reader, stdout, fmt.Sprintf("Default voice name [%s]: ", spec.DefaultVoice))
	if err != nil {
		return fmt.Errorf("read voice: %w", err)
	}
	voice := strings.TrimSpace(inputVoice)
	if voice == "" {
		voice = spec.DefaultVoice
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Validating API key and voice configuration...")

	profile := config.Profile{
		Provider:    spec.Name,
		Name:        profileName,
		Credentials: creds,
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
		appCfg.ActiveProvider = spec.Name
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
	for _, line := range credentialLines(profile) {
		fmt.Fprintf(stdout, "  %-10s %s\n", line.label+":", line.value)
	}
	if appCfg.ActiveProvider == spec.Name && appCfg.ActiveProfile == profileName {
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

// chooseSetupProvider picks the provider for setup. With a single implemented
// provider it is used silently; otherwise the user is asked.
func chooseSetupProvider(reader *bufio.Reader, stdout io.Writer) (tts.Spec, error) {
	names := providerNames()
	if len(names) == 0 {
		return tts.Spec{}, fmt.Errorf("no text-to-speech providers are available in this build")
	}
	if len(names) == 1 {
		spec, ok := lookupSpec(names[0])
		if !ok {
			return tts.Spec{}, fmt.Errorf("provider %q is registered but has no spec", names[0])
		}
		return spec, nil
	}

	defaultName := names[0]
	for _, name := range names {
		if name == "gcp" {
			defaultName = name
			break
		}
	}
	fmt.Fprintf(stdout, "Available providers: %s\n", strings.Join(names, ", "))
	input, err := promptLine(reader, stdout, fmt.Sprintf("Provider [%s]: ", defaultName))
	if err != nil {
		return tts.Spec{}, fmt.Errorf("read provider: %w", err)
	}
	name := strings.ToLower(strings.TrimSpace(input))
	if name == "" {
		name = defaultName
	}
	if err := validateProviderName(name); err != nil {
		return tts.Spec{}, err
	}
	spec, ok := lookupSpec(name)
	if !ok {
		return tts.Spec{}, fmt.Errorf("provider %q is registered but has no spec", name)
	}
	return spec, nil
}

// promptCredentials asks for every credential the provider declares. Secret
// fields use the masked password prompt.
func promptCredentials(reader *bufio.Reader, stdout io.Writer, spec tts.Spec) (map[string]interface{}, error) {
	creds := make(map[string]interface{}, len(spec.Credentials))
	for _, field := range spec.Credentials {
		prompt := field.Prompt
		if strings.TrimSpace(prompt) == "" {
			prompt = fmt.Sprintf("Enter your %s %s: ", spec.DisplayName, strings.ToLower(credentialLabel(field.Key)))
		}
		var (
			input string
			err   error
		)
		if field.Secret {
			input, err = promptPassword(reader, stdout, prompt)
		} else {
			input, err = promptLine(reader, stdout, prompt)
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", strings.ToLower(credentialLabel(field.Key)), err)
		}
		value := strings.TrimSpace(input)
		if value == "" {
			return nil, missingCredentialError(field, true)
		}
		creds[field.Key] = value
	}
	return creds, nil
}

// missingCredentialError explains how to supply a required credential.
// interactive selects the wording for setup; otherwise the flag form is named.
func missingCredentialError(field tts.CredentialField, interactive bool) error {
	label := credentialLabel(field.Key)
	if label == "API Key" {
		label = "API key"
	}
	var how string
	switch {
	case interactive:
		how = fmt.Sprintf("%s is required", label)
	case field.Key == "apiKey":
		how = "--api-key is required"
	default:
		how = fmt.Sprintf("--credential %s=VALUE is required", field.Key)
	}
	if strings.TrimSpace(field.Help) != "" {
		return fmt.Errorf("%s. %s", how, field.Help)
	}
	return fmt.Errorf("%s", how)
}
