package tts

import "context"

// Provider is the capability every text-to-speech backend exposes to the app.
type Provider interface {
	Name() string
	SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error)
	ListVoices(ctx context.Context, langCode string) ([]Voice, error)
	DefaultVoice(langCode string) string
}

// AudioEncodingMP3 is the only output encoding ttscli requests from providers.
const AudioEncodingMP3 = "MP3"

// Voice describes one synthesizer voice as reported by a provider.
type Voice struct {
	LanguageCodes []string `json:"languageCodes"`
	Name          string   `json:"name"`
	SsmlGender    string   `json:"ssmlGender"`
}

type SynthRequest struct {
	Text          string
	LanguageCode  string
	VoiceName     string
	AudioEncoding string
}
