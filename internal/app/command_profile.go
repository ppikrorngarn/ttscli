package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
)

func runProfileCommand(cfg cli.Config, stdout io.Writer) error {
	switch cfg.DefaultSubcommand {
	case cli.ProfileList:
		return runProfileList(stdout)
	case cli.ProfileCreate:
		return runProfileCreate(cfg, stdout)
	case cli.ProfileDelete:
		return runProfileDelete(cfg, stdout)
	case cli.ProfileUse:
		return runProfileUse(cfg, stdout)
	case cli.ProfileGet:
		return runProfileGet(cfg, stdout)
	default:
		return fmt.Errorf("unsupported profile subcommand: %s", cfg.DefaultSubcommand)
	}
}

func runProfileList(stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if len(appCfg.Profiles) == 0 {
		fmt.Fprintln(stdout, "No profiles configured yet.")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Create your first profile with one of these commands:")
		fmt.Fprintln(stdout, "  ttscli setup                              # Interactive setup wizard")
		fmt.Fprintln(stdout, "  ttscli profile create --provider gcp --name default --api-key YOUR_KEY")
		return nil
	}

	fmt.Fprintln(stdout, "Configured profiles:")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "PROFILE KEY      PROVIDER   NAME       DEFAULT VOICE        LANG")
	fmt.Fprintln(stdout, "──────────────── ───────── ───────── ───────────────────── ────────")
	for key, profile := range appCfg.Profiles {
		isActive := ""
		if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
			isActive = " ✓"
		}
		defaultVoice := profile.Defaults["voice"]
		if defaultVoice == "" {
			defaultVoice = "(not set)"
		}
		defaultLang := profile.Defaults["lang"]
		if defaultLang == "" {
			defaultLang = "(not set)"
		}
		fmt.Fprintf(stdout, "%-16s %-9s %-9s %-21s %-8s%s\n",
			key, profile.Provider, profile.Name, defaultVoice, defaultLang, isActive)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Legend: ✓ = active profile")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Manage profiles:")
	fmt.Fprintln(stdout, "  ttscli profile create --provider <provider> --name <name> --api-key <key>")
	fmt.Fprintln(stdout, "  ttscli profile use <provider:name>")
	fmt.Fprintln(stdout, "  ttscli profile get <provider:name>")
	fmt.Fprintln(stdout, "  ttscli profile delete <provider:name>")
	return nil
}

func runProfileCreate(cfg cli.Config, stdout io.Writer) error {
	if strings.TrimSpace(cfg.Provider) == "" {
		return fmt.Errorf("--provider is required. Specify the TTS provider (gcp, aws, azure, ibm, alibaba)")
	}
	if strings.TrimSpace(cfg.ProfileName) == "" {
		return fmt.Errorf("--name is required. Choose a unique name for this profile")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("--api-key is required. Get your API key from the provider's console")
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(cfg.Provider))

	profileKey, providerName, profileName, err := config.BuildProfileKey(normalizedProvider, cfg.ProfileName)
	if err != nil {
		return err
	}

	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if _, exists := appCfg.Profiles[profileKey]; exists {
		return fmt.Errorf("profile %q already exists. Choose a different name or delete it first", profileKey)
	}

	profile := config.Profile{
		Provider: providerName,
		Name:     profileName,
		Credentials: map[string]interface{}{
			"apiKey": cfg.APIKey,
		},
		Defaults: make(map[string]string),
	}

	if cfg.DefaultVoice != "" {
		profile.Defaults["voice"] = cfg.DefaultVoice
	}

	fmt.Fprintln(stdout, "Validating provider credentials...")

	provider, err := newProvider(profile)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	ctx, stop := newAppCtx()
	defer stop()

	voices, err := provider.ListVoices(ctx, "en-US")
	if err != nil {
		return fmt.Errorf("validate provider: %w", err)
	}
	if len(voices) == 0 {
		return fmt.Errorf("provider returned no voices. Please check your API key and permissions")
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = profile.Provider
		appCfg.ActiveProfile = profile.Name
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "✓ Profile created: %s\n", profileKey)
	if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
		fmt.Fprintf(stdout, "✓ Profile set as active.\n")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintf(stdout, "  ttscli speak --text \"Hello\" --profile %s\n", profileKey)
	fmt.Fprintf(stdout, "  ttscli profile use %s\n", profileKey)
	return nil
}

func runProfileDelete(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profileKey, _, _, err := config.ParseProfileKey(cfg.Profile)
	if err != nil {
		return err
	}

	if _, exists := appCfg.Profiles[profileKey]; !exists {
		return fmt.Errorf("profile %q not found. Run 'ttscli profile list' to see available profiles", profileKey)
	}

	delete(appCfg.Profiles, profileKey)

	if appCfg.ActiveProvider+":"+appCfg.ActiveProfile == profileKey {
		appCfg.ActiveProvider = ""
		appCfg.ActiveProfile = ""
		for key, profile := range appCfg.Profiles {
			appCfg.ActiveProvider = profile.Provider
			appCfg.ActiveProfile = profile.Name
			fmt.Fprintf(stdout, "Switched active profile to: %s\n", key)
			break
		}
		if appCfg.ActiveProvider == "" {
			fmt.Fprintln(stdout, "No active profile set. Use 'ttscli profile use' to set one.")
		}
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "✓ Profile deleted: %s\n", profileKey)
	return nil
}

func runProfileUse(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profileKey, provider, name, err := config.ParseProfileKey(cfg.Profile)
	if err != nil {
		return err
	}

	if _, exists := appCfg.Profiles[profileKey]; !exists {
		return fmt.Errorf("profile %q not found. Run 'ttscli profile list' to see available profiles", profileKey)
	}

	appCfg.ActiveProvider = provider
	appCfg.ActiveProfile = name

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "✓ Active profile set to: %s\n", profileKey)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "You can now use TTS commands without specifying --profile:")
	fmt.Fprintln(stdout, "  ttscli speak --text \"Hello world\"")
	fmt.Fprintln(stdout, "  ttscli voices")
	return nil
}

func runProfileGet(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profileKey, _, _, err := config.ParseProfileKey(cfg.Profile)
	if err != nil {
		return err
	}

	profile, exists := appCfg.Profiles[profileKey]
	if !exists {
		return fmt.Errorf("profile %q not found. Run 'ttscli profile list' to see available profiles", profileKey)
	}

	isActive := ""
	if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
		isActive = " (active)"
	}

	fmt.Fprintln(stdout, "Profile Details")
	fmt.Fprintln(stdout, "───────────────")
	fmt.Fprintf(stdout, "Profile Key:   %s%s\n", profileKey, isActive)
	fmt.Fprintf(stdout, "Provider:      %s\n", profile.Provider)
	fmt.Fprintf(stdout, "Name:          %s\n", profile.Name)
	fmt.Fprintf(stdout, "API Key:       %s\n", maskAPIKey(resolveProfileAPIKey(profile)))
	if voice, ok := profile.Defaults["voice"]; ok {
		fmt.Fprintf(stdout, "Default Voice: %s\n", voice)
	} else {
		fmt.Fprintln(stdout, "Default Voice: (not set)")
	}
	if lang, ok := profile.Defaults["lang"]; ok {
		fmt.Fprintf(stdout, "Default Language: %s\n", lang)
	} else {
		fmt.Fprintln(stdout, "Default Language: (not set)")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintf(stdout, "  ttscli speak --text \"Hello\" --profile %s\n", profileKey)
	fmt.Fprintf(stdout, "  ttscli save --text \"Hello\" --out speech.mp3 --profile %s\n", profileKey)
	return nil
}
