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

func (c *Client) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
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

func (c *Client) buildRequestURL(path string, extraParams url.Values) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("key", c.apiKey)
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

func (c *Client) do(req *http.Request) ([]byte, error) {
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
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}
	return body, nil
}

func PrintVoices(w io.Writer, langCode string, voices []Voice) {
	fmt.Fprintf(w, "Available voices for language: %s\n", langCode)
	fmt.Fprintf(w, "%-35s | %-10s | %s\n", "VOICE NAME", "GENDER", "LANGUAGES")
	fmt.Fprintln(w, "----------------------------------------------------------------------")
	for _, v := range voices {
		fmt.Fprintf(w, "%-35s | %-10s | %v\n", v.Name, v.SsmlGender, v.LanguageCodes)
	}
}
