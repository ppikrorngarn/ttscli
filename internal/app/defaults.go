package app

import (
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
)

// resolveProfileDefaults applies profile defaults to the config when flags are not provided.
func resolveProfileDefaults(cfg cli.Config, profile config.Profile) cli.Config {
	if !cfg.HasVoiceFlag && profile.Defaults["voice"] != "" {
		cfg.Voice = profile.Defaults["voice"]
	}
	if !cfg.HasLangFlag && profile.Defaults["lang"] != "" {
		cfg.Lang = profile.Defaults["lang"]
	}
	return cfg
}

// resolveProfileAPIKey extracts API key from profile credentials.
func resolveProfileAPIKey(profile config.Profile) string {
	if apiKey, ok := profile.Credentials["apiKey"].(string); ok {
		return strings.TrimSpace(apiKey)
	}
	return ""
}

// maskAPIKey hides all but the last four characters of the key for display.
func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(not set)"
	}
	if len(trimmed) <= 4 {
		return "****"
	}
	return "****" + trimmed[len(trimmed)-4:]
}
