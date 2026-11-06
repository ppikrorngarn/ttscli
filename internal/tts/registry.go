package tts

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// CredentialField describes one value a provider needs in Profile.Credentials.
type CredentialField struct {
	// Key is the map key stored in the profile, for example "apiKey".
	Key string
	// Prompt is shown by interactive setup when asking for the value.
	Prompt string
	// Help says where to obtain the value. It is shown when the value is missing.
	Help string
	// Secret values are read with a masked prompt and masked in output.
	Secret bool
}

// Spec is everything the app layer needs to know about a provider without
// importing its implementation: how to prompt for credentials, what defaults
// to offer, and how to construct it.
type Spec struct {
	// Name is the short identifier used in profile keys, for example "gcp".
	Name string
	// DisplayName is the human-readable service name.
	DisplayName string
	// Credentials lists every value the provider requires, in prompt order.
	Credentials []CredentialField
	// DefaultLang and DefaultVoice are offered by setup and used as fallbacks.
	DefaultLang  string
	DefaultVoice string
	// New constructs the provider. creds holds one entry per CredentialField.
	// A nil httpClient means "use the provider's default".
	New func(creds map[string]string, httpClient *http.Client) (Provider, error)
}

var gcpSpec = Spec{
	Name:        "gcp",
	DisplayName: "Google Cloud Text-to-Speech",
	Credentials: []CredentialField{
		{
			Key:    "apiKey",
			Prompt: "Enter your Google Cloud API key: ",
			Help:   "Create one in Google Cloud Console under APIs & Services > Credentials and restrict it to the Cloud Text-to-Speech API",
			Secret: true,
		},
	},
	DefaultLang:  "en-US",
	DefaultVoice: "en-US-Neural2-F",
	New: func(creds map[string]string, httpClient *http.Client) (Provider, error) {
		return NewGCPClient(creds["apiKey"], httpClient), nil
	},
}

// registry holds every implemented provider keyed by Spec.Name.
var registry = map[string]Spec{
	alibabaSpec.Name: alibabaSpec,
	azureSpec.Name:   azureSpec,
	gcpSpec.Name:     gcpSpec,
	ibmSpec.Name:     ibmSpec,
}

// plannedProviders are advertised but not implemented yet. They get a
// friendlier error than an unknown name.
var plannedProviders = []string{"aws"}

// Lookup returns the Spec registered under name.
func Lookup(name string) (Spec, bool) {
	spec, ok := registry[name]
	return spec, ok
}

// Names returns the implemented provider names in sorted order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Planned returns the provider names that are announced but not implemented.
func Planned() []string {
	out := make([]string, len(plannedProviders))
	copy(out, plannedProviders)
	return out
}

// IsPlanned reports whether name is an announced, unimplemented provider.
func IsPlanned(name string) bool {
	for _, p := range plannedProviders {
		if p == name {
			return true
		}
	}
	return false
}

// ValidateName returns nil when name is an implemented provider and a
// user-facing error otherwise.
func ValidateName(name string) error {
	if _, ok := registry[name]; ok {
		return nil
	}
	available := strings.Join(Names(), ", ")
	if IsPlanned(name) {
		return fmt.Errorf("provider %q is not yet implemented. Available today: %s", name, available)
	}
	return fmt.Errorf("unsupported provider %q. Supported providers: %s", name, available)
}

// NewProvider builds the provider named by providerName from the raw
// credentials map stored in a profile. Every CredentialField declared by the
// provider must be present as a non-empty string.
func NewProvider(providerName string, creds map[string]interface{}) (Provider, error) {
	if err := ValidateName(providerName); err != nil {
		return nil, err
	}
	spec := registry[providerName]

	resolved, err := ResolveCredentials(spec, creds)
	if err != nil {
		return nil, err
	}
	return spec.New(resolved, nil)
}

// ResolveCredentials checks the raw credentials map against spec.Credentials
// and returns a string map with exactly the declared fields.
func ResolveCredentials(spec Spec, creds map[string]interface{}) (map[string]string, error) {
	resolved := make(map[string]string, len(spec.Credentials))
	for _, field := range spec.Credentials {
		raw, ok := creds[field.Key]
		if !ok {
			return nil, missingCredential(spec, field)
		}
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s profile credential %q must be a string", spec.Name, field.Key)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, missingCredential(spec, field)
		}
		resolved[field.Key] = value
	}
	return resolved, nil
}

func missingCredential(spec Spec, field CredentialField) error {
	if field.Help != "" {
		return fmt.Errorf("%s profile is missing credential %q. %s", spec.Name, field.Key, field.Help)
	}
	return fmt.Errorf("%s profile is missing credential %q", spec.Name, field.Key)
}
