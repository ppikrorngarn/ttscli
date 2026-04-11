package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	GoogleTTSBaseURL   = "https://texttospeech.googleapis.com"
	defaultHTTPTimeout = 30 * time.Second
	apiPathSynthesize  = "/v1/text:synthesize"
	apiPathVoices      = "/v1/voices"
	AudioEncodingMP3   = "MP3"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type synthesizeRequest struct {
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Voice struct {
		LanguageCode string `json:"languageCode"`
		Name         string `json:"name,omitempty"`
		SsmlGender   string `json:"ssmlGender,omitempty"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding string  `json:"audioEncoding"`
		SpeakingRate  float64 `json:"speakingRate,omitempty"`
		Pitch         float64 `json:"pitch,omitempty"`
	} `json:"audioConfig"`
}

type synthesizeResponse struct {
	AudioContent string `json:"audioContent"`
}

type listVoicesResponse struct {
	Voices []Voice `json:"voices"`
}

type Voice struct {
	LanguageCodes []string `json:"languageCodes"`
	Name          string   `json:"name"`
	SsmlGender    string   `json:"ssmlGender"`
}

func NewClient(apiKey string, httpClient *http.Client) *Client {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		baseURL:    GoogleTTSBaseURL,
		apiKey:     apiKey,
		httpClient: client,
	}
}

func (c *Client) Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
	var reqBody synthesizeRequest
	reqBody.Input.Text = text
	reqBody.Voice.LanguageCode = languageCode
	reqBody.Voice.Name = voiceName
	reqBody.AudioConfig.AudioEncoding = audioEncoding

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + apiPathSynthesize + "?key=" + c.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed synthesizeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if parsed.AudioContent == "" {
		return nil, fmt.Errorf("empty audioContent in response")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(parsed.AudioContent)
	if err != nil {
		return nil, fmt.Errorf("decode audioContent: %w", err)
	}
	return audioBytes, nil
}

func (c *Client) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
	url := c.baseURL + apiPathVoices + "?key=" + c.apiKey
	if langCode != "" {
		url += "&languageCode=" + langCode
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}

	var parsed listVoicesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return parsed.Voices, nil
}

func PrintVoices(w io.Writer, langCode string, voices []Voice) {
	fmt.Fprintf(w, "Available voices for language: %s\n", langCode)
	fmt.Fprintf(w, "%-35s | %-10s | %s\n", "VOICE NAME", "GENDER", "LANGUAGES")
	fmt.Fprintln(w, "----------------------------------------------------------------------")
	for _, v := range voices {
		fmt.Fprintf(w, "%-35s | %-10s | %v\n", v.Name, v.SsmlGender, v.LanguageCodes)
	}
}
