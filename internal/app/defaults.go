package app

import (
	"sort"
	"strings"
	"unicode"

	"github.com/ppikrorngarn/ttscli/internal/cli"
	"github.com/ppikrorngarn/ttscli/internal/config"
	"github.com/ppikrorngarn/ttscli/internal/tts"
)

// resolveProfileDefaults fills in language and voice when flags were not given:
// profile defaults first, then the provider's own default voice for the
// resolved language. A nil provider skips that last step.
func resolveProfileDefaults(cfg cli.Config, profile config.Profile, provider tts.Provider) cli.Config {
	if !cfg.HasVoiceFlag && profile.Defaults["voice"] != "" {
		cfg.Voice = profile.Defaults["voice"]
	}
	if !cfg.HasLangFlag && profile.Defaults["lang"] != "" {
		cfg.Lang = profile.Defaults["lang"]
	}
	if cfg.Voice == "" && provider != nil {
		cfg.Voice = provider.DefaultVoice(cfg.Lang)
	}
	return cfg
}

// credentialLabel turns a credential key into a display label: "apiKey" -> "API Key",
// "region" -> "Region", "subscriptionKey" -> "Subscription Key".
func credentialLabel(key string) string {
	if key == "apiKey" {
		return "API Key"
	}
	var b strings.Builder
	for i, r := range key {
		if i == 0 {
			b.WriteRune(unicode.ToUpper(r))
			continue
		}
		if unicode.IsUpper(r) {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// credentialLine is one "Label: value" pair ready for display.
type credentialLine struct {
	label string
	value string
}

// credentialLines renders every credential the provider declares, masking
// secrets. For a provider the registry does not know, every value is masked.
func credentialLines(profile config.Profile) []credentialLine {
	spec, known := lookupSpec(profile.Provider)
	if !known {
		keys := make([]string, 0, len(profile.Credentials))
		for k := range profile.Credentials {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines := make([]credentialLine, 0, len(keys))
		for _, k := range keys {
			lines = append(lines, credentialLine{credentialLabel(k), maskAPIKey(credentialString(profile, k))})
		}
		return lines
	}
	lines := make([]credentialLine, 0, len(spec.Credentials))
	for _, field := range spec.Credentials {
		value := credentialString(profile, field.Key)
		switch {
		case field.Secret:
			value = maskAPIKey(value)
		case strings.TrimSpace(value) == "":
			value = "(not set)"
		}
		lines = append(lines, credentialLine{credentialLabel(field.Key), value})
	}
	return lines
}

// credentialString returns the trimmed string value stored under key, or "".
func credentialString(profile config.Profile, key string) string {
	if raw, ok := profile.Credentials[key].(string); ok {
		return strings.TrimSpace(raw)
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
