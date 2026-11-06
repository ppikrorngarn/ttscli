package tts

// Azure Speech (Microsoft Cognitive Services text to speech) provider.
//
// Implemented against the REST API documentation as of 2026-11-06:
//   - https://learn.microsoft.com/azure/ai-services/speech-service/rest-text-to-speech
//     (authentication headers, GET .../cognitiveservices/voices/list response
//     shape, POST .../cognitiveservices/v1 headers, SSML body, status codes,
//     X-Microsoft-OutputFormat values)
//   - https://learn.microsoft.com/azure/ai-services/speech-service/speech-synthesis-markup-structure
//     (speak/voice element structure)
//   - https://learn.microsoft.com/azure/ai-services/speech-service/regions
//     (regional https://{region}.tts.speech.microsoft.com hosts)

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	// AzureTTSHostTemplate is the regional text-to-speech host. %s is the
	// Azure region of the Speech resource, for example "eastus".
	AzureTTSHostTemplate = "https://%s.tts.speech.microsoft.com"

	azurePathSynthesize  = "/cognitiveservices/v1"
	azurePathVoices      = "/cognitiveservices/voices/list"
	azureOutputFormatMP3 = "audio-24khz-96kbitrate-mono-mp3"
	azureUserAgent       = "ttscli"
	azureSSMLNamespace   = "http://www.w3.org/2001/10/synthesis"

	// azureErrorBodyPreview caps how much of an unexpected response body is
	// echoed back in an error message.
	azureErrorBodyPreview = 200
)

var azureSpec = Spec{
	Name:        "azure",
	DisplayName: "Azure Speech",
	Credentials: []CredentialField{
		{
			Key:    "subscriptionKey",
			Prompt: "Enter your Azure Speech resource key: ",
			Help:   "Copy KEY 1 or KEY 2 from the Azure portal: your Speech resource > Keys and Endpoint",
			Secret: true,
		},
		{
			Key:    "region",
			Prompt: "Enter the Azure region of the Speech resource (e.g. eastus): ",
			Help:   "Use the Location/Region shown next to the keys in the Azure portal, e.g. eastus, westeurope, southeastasia",
			Secret: false,
		},
	},
	DefaultLang:  "en-US",
	DefaultVoice: "en-US-JennyNeural",
	New: func(creds map[string]string, httpClient *http.Client) (Provider, error) {
		region, err := normalizeAzureRegion(creds["region"])
		if err != nil {
			return nil, err
		}
		return NewAzureClient(creds["subscriptionKey"], region, httpClient), nil
	},
}

// AzureClient talks to the Azure Speech text-to-speech REST API with a
// resource key (Ocp-Apim-Subscription-Key header).
type AzureClient struct {
	// baseURL is the scheme and host every request is built on. It defaults to
	// AzureTTSHostTemplate filled with the region; tests point it at an
	// httptest server.
	baseURL         string
	subscriptionKey string
	region          string
	httpClient      *http.Client
}

// azureVoice is one entry of the voices/list response. Only the fields the
// app needs are decoded; the service returns many more.
type azureVoice struct {
	Name                string   `json:"Name"`
	ShortName           string   `json:"ShortName"`
	Gender              string   `json:"Gender"`
	Locale              string   `json:"Locale"`
	SecondaryLocaleList []string `json:"SecondaryLocaleList"`
}

// NewAzureClient returns an Azure Speech provider for the given resource key
// and region. A nil httpClient selects a default client with a 30 second
// timeout.
func NewAzureClient(subscriptionKey, region string, httpClient *http.Client) *AzureClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &AzureClient{
		baseURL:         fmt.Sprintf(AzureTTSHostTemplate, region),
		subscriptionKey: subscriptionKey,
		region:          region,
		httpClient:      client,
	}
}

func (c *AzureClient) Name() string {
	return azureSpec.Name
}

// DefaultVoice returns the spec default for the spec's default language and
// "" otherwise. Azure requires a voice name in every request, so for other
// languages the caller has to pick one with --voice (see SynthesizeRequest).
func (c *AzureClient) DefaultVoice(langCode string) string {
	if strings.EqualFold(langCode, azureSpec.DefaultLang) {
		return azureSpec.DefaultVoice
	}
	return ""
}

func (c *AzureClient) SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error) {
	if req.AudioEncoding != "" && req.AudioEncoding != AudioEncodingMP3 {
		return nil, fmt.Errorf("unsupported audio encoding %q: azure provider only produces %s", req.AudioEncoding, AudioEncodingMP3)
	}

	voice := req.VoiceName
	if voice == "" {
		voice = c.DefaultVoice(req.LanguageCode)
	}
	if voice == "" {
		return nil, fmt.Errorf("azure requires a voice name for language %q; pass --voice (list options with the voices command)", req.LanguageCode)
	}

	lang := req.LanguageCode
	if lang == "" {
		lang = azureLocaleFromVoice(voice)
	}

	ssml := buildAzureSSML(lang, voice, req.Text)

	reqURL, err := c.buildRequestURL(azurePathSynthesize)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(ssml))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/ssml+xml")
	httpReq.Header.Set("X-Microsoft-OutputFormat", azureOutputFormatMP3)

	body, contentType, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty audio in response")
	}
	if !azureIsAudioContentType(contentType) {
		return nil, fmt.Errorf("unexpected non-audio response (content-type %q): %s", contentType, azurePreview(body))
	}
	return body, nil
}

func (c *AzureClient) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
	reqURL, err := c.buildRequestURL(azurePathVoices)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	body, _, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var parsed []azureVoice
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	voices := make([]Voice, 0, len(parsed))
	for _, v := range parsed {
		voice := azureToVoice(v)
		if voice.Name == "" {
			continue
		}
		if langCode != "" && !azureSpeaks(voice, langCode) {
			continue
		}
		voices = append(voices, voice)
	}
	sort.Slice(voices, func(i, j int) bool { return voices[i].Name < voices[j].Name })
	return voices, nil
}

func (c *AzureClient) buildRequestURL(path string) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(&url.URL{Path: path}).String(), nil
}

// do sends req with the auth and User-Agent headers, returning the body and
// the response Content-Type on 2xx and a status error otherwise.
func (c *AzureClient) do(req *http.Request) ([]byte, string, error) {
	req.Header.Set("Ocp-Apim-Subscription-Key", c.subscriptionKey)
	// The service rejects requests without a User-Agent.
	req.Header.Set("User-Agent", azureUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", formatAzureAPIError(resp.StatusCode, body)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// formatAzureAPIError builds the status error. Azure error bodies are usually
// empty, sometimes plain text, occasionally JSON; all three are handled.
func formatAzureAPIError(statusCode int, body []byte) error {
	message := extractAPIErrorMessage(body)
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		return fmt.Errorf("tts request failed with status %d", statusCode)
	}
	return fmt.Errorf("tts request failed with status %d: %s", statusCode, azurePreview([]byte(message)))
}

// buildAzureSSML renders the request body. Every interpolated value is
// XML-escaped; xml.EscapeText also escapes quotes, so attribute values are
// safe inside single quotes.
func buildAzureSSML(lang, voice, text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("<speak version='1.0' xmlns='")
	buf.WriteString(azureSSMLNamespace)
	buf.WriteString("' xml:lang='")
	_ = xml.EscapeText(&buf, []byte(lang))
	buf.WriteString("'><voice name='")
	_ = xml.EscapeText(&buf, []byte(voice))
	buf.WriteString("'>")
	_ = xml.EscapeText(&buf, []byte(text))
	buf.WriteString("</voice></speak>")
	return buf.Bytes()
}

// azureLocaleFromVoice derives "en-US" from "en-US-JennyNeural" for requests
// that carry a voice but no language. Falls back to the spec default.
func azureLocaleFromVoice(voice string) string {
	parts := strings.SplitN(voice, "-", 3)
	if len(parts) == 3 && parts[0] != "" && parts[1] != "" {
		return parts[0] + "-" + parts[1]
	}
	return azureSpec.DefaultLang
}

func azureToVoice(v azureVoice) Voice {
	codes := make([]string, 0, 1+len(v.SecondaryLocaleList))
	if v.Locale != "" {
		codes = append(codes, v.Locale)
	}
	for _, extra := range v.SecondaryLocaleList {
		if extra != "" && !strings.EqualFold(extra, v.Locale) {
			codes = append(codes, extra)
		}
	}
	return Voice{
		Name:          v.ShortName,
		LanguageCodes: codes,
		SsmlGender:    strings.ToUpper(strings.TrimSpace(v.Gender)),
	}
}

// azureSpeaks reports whether any of the voice's locales equals langCode,
// ignoring case. The service has no server-side language filter.
func azureSpeaks(v Voice, langCode string) bool {
	for _, code := range v.LanguageCodes {
		if strings.EqualFold(code, langCode) {
			return true
		}
	}
	return false
}

func azureIsAudioContentType(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(contentType))
	}
	return strings.HasPrefix(mediaType, "audio/") || mediaType == "application/octet-stream"
}

func azurePreview(body []byte) string {
	runes := []rune(strings.TrimSpace(string(body)))
	if len(runes) > azureErrorBodyPreview {
		return string(runes[:azureErrorBodyPreview]) + "..."
	}
	return string(runes)
}

// normalizeAzureRegion lower-cases and validates the region so it can be
// placed in a host name without surprises (regions are short alphanumeric
// identifiers such as "eastus" or "southeastasia").
func normalizeAzureRegion(region string) (string, error) {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		return "", fmt.Errorf("azure region must not be empty")
	}
	for _, r := range region {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit {
			return "", fmt.Errorf("invalid azure region %q: expected letters and digits only, e.g. eastus", region)
		}
	}
	return region, nil
}
