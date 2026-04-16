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
		fmt.Fprintln(stdout, "No profiles found. Create one with: ttscli profile create")
		return nil
	}

	fmt.Fprintln(stdout, "Available profiles:")
	fmt.Fprintln(stdout, "PROFILE KEY   | PROVIDER | NAME    | DEFAULT VOICE | DEFAULT LANG")
	fmt.Fprintln(stdout, "-------------------------------------------------------------------")
	for key, profile := range appCfg.Profiles {
		isActive := ""
		if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
			isActive = " (active)"
		}
		defaultVoice := profile.Defaults["voice"]
		if defaultVoice == "" {
			defaultVoice = "(not set)"
		}
		defaultLang := profile.Defaults["lang"]
		if defaultLang == "" {
			defaultLang = "(not set)"
		}
		fmt.Fprintf(stdout, "%-13s | %-8s | %-7s | %-13s | %s%s\n",
			key, profile.Provider, profile.Name, defaultVoice, defaultLang, isActive)
	}
	return nil
}

func runProfileCreate(cfg cli.Config, stdout io.Writer) error {
	if cfg.Lang == "" {
		return fmt.Errorf("--provider is required")
	}
	if cfg.Voice == "" {
		return fmt.Errorf("--name is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("--api-key is required")
	}

	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profileKey := cfg.Lang + ":" + cfg.Voice
	if _, exists := appCfg.Profiles[profileKey]; exists {
		return fmt.Errorf("profile %q already exists", profileKey)
	}

	profile := config.Profile{
		Provider: cfg.Lang,
		Name:     cfg.Voice,
		Credentials: map[string]interface{}{
			"apiKey": cfg.APIKey,
		},
		Defaults: make(map[string]string),
	}

	if cfg.SavePath != "" {
		profile.Defaults["voice"] = cfg.SavePath
	}

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
		return fmt.Errorf("provider returned no voices, validation failed")
	}

	appCfg.Profiles[profileKey] = profile
	if appCfg.ActiveProvider == "" && appCfg.ActiveProfile == "" {
		appCfg.ActiveProvider = profile.Provider
		appCfg.ActiveProfile = profile.Name
	}

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "Profile created: %s\n", profileKey)
	if profile.Provider == appCfg.ActiveProvider && profile.Name == appCfg.ActiveProfile {
		fmt.Fprintf(stdout, "Profile set as active.\n")
	}
	return nil
}

func runProfileDelete(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if _, exists := appCfg.Profiles[cfg.Profile]; !exists {
		return fmt.Errorf("profile %q not found", cfg.Profile)
	}

	delete(appCfg.Profiles, cfg.Profile)

	if appCfg.ActiveProvider+":"+appCfg.ActiveProfile == cfg.Profile {
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

	fmt.Fprintf(stdout, "Profile deleted: %s\n", cfg.Profile)
	return nil
}

func runProfileUse(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	parts := strings.Split(cfg.Profile, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid profile key format, expected provider:name (e.g., gcp:default)")
	}

	provider := parts[0]
	name := parts[1]
	profileKey := cfg.Profile

	if _, exists := appCfg.Profiles[profileKey]; !exists {
		return fmt.Errorf("profile %q not found", profileKey)
	}

	appCfg.ActiveProvider = provider
	appCfg.ActiveProfile = name

	if err := saveConfig(appCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(stdout, "Active profile set to: %s\n", profileKey)
	return nil
}

func runProfileGet(cfg cli.Config, stdout io.Writer) error {
	appCfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profile, err := getProfile(appCfg, cfg.Profile)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Profile: %s\n", cfg.Profile)
	fmt.Fprintf(stdout, "Provider: %s\n", profile.Provider)
	fmt.Fprintf(stdout, "Name: %s\n", profile.Name)
	fmt.Fprintf(stdout, "API Key: %s\n", maskAPIKey(resolveProfileAPIKey(profile)))
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
	return nil
}
