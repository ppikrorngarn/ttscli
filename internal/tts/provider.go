package tts

import (
	"context"
	"fmt"
)

type Provider interface {
	Name() string
	SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error)
	ListVoices(ctx context.Context, langCode string) ([]Voice, error)
	DefaultVoice(langCode string) string
}

type SynthRequest struct {
	Text          string
	LanguageCode  string
	VoiceName     string
	AudioEncoding string
}

func NewProvider(providerName string, creds interface{}) (Provider, error) {
	switch providerName {
	case "gcp":
		if apiKey, ok := creds.(string); ok {
			return NewGCPClient(apiKey, nil), nil
		}
		return nil, fmt.Errorf("gcp provider requires string API key as credentials")
	case "aws":
		return nil, fmt.Errorf("aws provider not yet implemented")
	case "azure":
		return nil, fmt.Errorf("azure provider not yet implemented")
	case "ibm":
		return nil, fmt.Errorf("ibm provider not yet implemented")
	case "alibaba":
		return nil, fmt.Errorf("alibaba provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
}
