package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	GoogleTTSBaseURL   = "https://texttospeech.googleapis.com"
	defaultHTTPTimeout = 30 * time.Second
	apiPathSynthesize  = "/v1/text:synthesize"
	apiPathVoices      = "/v1/voices"
)

type GCPClient struct {
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

// NewGCPClient returns a Google Cloud Text-to-Speech provider authenticated with an API key.
// A nil httpClient selects a default client with a 30 second timeout.
func NewGCPClient(apiKey string, httpClient *http.Client) *GCPClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &GCPClient{
		baseURL:    GoogleTTSBaseURL,
		apiKey:     apiKey,
		httpClient: client,
	}
}

func (c *GCPClient) Name() string {
	return "gcp"
}

func (c *GCPClient) SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error) {
	return c.Synthesize(ctx, req.Text, req.LanguageCode, req.VoiceName, req.AudioEncoding)
}

func (c *GCPClient) DefaultVoice(langCode string) string {
	if langCode == "en-US" {
		return "en-US-Neural2-F"
	}
	return ""
}

func (c *GCPClient) Synthesize(ctx context.Context, text, languageCode, voiceName, audioEncoding string) ([]byte, error) {
	var reqBody synthesizeRequest
	reqBody.Input.Text = text
	reqBody.Voice.LanguageCode = languageCode
	reqBody.Voice.Name = voiceName
	reqBody.AudioConfig.AudioEncoding = audioEncoding

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqURL, err := c.buildRequestURL(apiPathSynthesize, url.Values{})
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	body, err := c.do(req)
	if err != nil {
		return nil, err
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

func (c *GCPClient) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
	extraParams := url.Values{}
	if langCode != "" {
		extraParams.Set("languageCode", langCode)
	}

	reqURL, err := c.buildRequestURL(apiPathVoices, extraParams)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var parsed listVoicesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return parsed.Voices, nil
}

func (c *GCPClient) buildRequestURL(path string, extraParams url.Values) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	for k, values := range extraParams {
		for _, v := range values {
			params.Add(k, v)
		}
	}

	ref := &url.URL{
		Path:     path,
		RawQuery: params.Encode(),
	}
	return baseURL.ResolveReference(ref).String(), nil
}

func (c *GCPClient) do(req *http.Request) ([]byte, error) {
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, formatAPIError(resp.StatusCode, body)
	}
	return body, nil
}

func formatAPIError(statusCode int, body []byte) error {
	message := extractAPIErrorMessage(body)
	if message == "" {
		return fmt.Errorf("tts request failed with status %d", statusCode)
	}
	return fmt.Errorf("tts request failed with status %d: %s", statusCode, message)
}

func extractAPIErrorMessage(body []byte) string {
	var parsed struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}

	if message := extractNestedErrorMessage(parsed.Error); message != "" {
		return message
	}
	for _, candidate := range []string{
		parsed.Message,
		stringValue(parsed.Error),
	} {
		if message := strings.TrimSpace(candidate); message != "" {
			return message
		}
	}
	return ""
}

func extractNestedErrorMessage(raw json.RawMessage) string {
	var nested struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &nested); err != nil {
		return ""
	}
	return strings.TrimSpace(nested.Message)
}

func stringValue(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}
