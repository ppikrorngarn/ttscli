package app

import (
	"strings"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
)

// resolveRunDefaults applies saved defaults to the config when flags are not provided.
func resolveRunDefaults(cfg cli.Config, defaults config.Defaults) cli.Config {
	if !cfg.HasVoiceFlag && strings.TrimSpace(defaults.Voice) != "" {
		cfg.Voice = defaults.Voice
	}
	if !cfg.HasLangFlag && strings.TrimSpace(defaults.Lang) != "" {
		cfg.Lang = defaults.Lang
	}
	return cfg
}

// resolveAPIKey prefers the environment variable over the saved API key.
func resolveAPIKey(savedAPIKey string) string {
	if envKey := strings.TrimSpace(lookupEnv(apiKeyEnvVar)); envKey != "" {
		return envKey
	}
	return strings.TrimSpace(savedAPIKey)
}

// defaultsEmpty reports whether all fields in Defaults are empty.
func defaultsEmpty(d config.Defaults) bool {
	return strings.TrimSpace(d.Voice) == "" &&
		strings.TrimSpace(d.Lang) == "" &&
		strings.TrimSpace(d.APIKey) == ""
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
