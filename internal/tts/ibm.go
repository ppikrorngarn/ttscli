// IBM Watson Text to Speech provider.
//
// Official documentation relied on (checked 2026-11-06):
//   - https://cloud.ibm.com/apidocs/text-to-speech
//     POST /v1/synthesize (voice query, Accept audio/mp3, body {"text"}),
//     GET /v1/voices, error body {"code","error","code_description"},
//     default voice en-US_MichaelV3Voice when voice is omitted.
//   - https://cloud.ibm.com/docs/text-to-speech?topic=text-to-speech-gettingStarted
//     curl -u "apikey:{apikey}" (HTTP Basic, user name "apikey"), Content-Type
//     application/json, {url}/v1/synthesize?voice=...
//   - https://cloud.ibm.com/docs/text-to-speech?topic=text-to-speech-voices-list
//     voices[] fields: name, language, gender ("male"/"female"), description,
//     customizable, supported_features, url.
//   - https://cloud.ibm.com/docs/text-to-speech?topic=text-to-speech-voices
//     voice names, languages and genders.

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	ibmPathSynthesize = "/v1/synthesize"
	ibmPathVoices     = "/v1/voices"
	// ibmBasicAuthUser is the fixed HTTP Basic user name IBM Cloud expects
	// when the password is an IAM API key.
	ibmBasicAuthUser = "apikey"
	// ibmAcceptMP3 is the Accept value that makes the service return MP3.
	ibmAcceptMP3 = "audio/mp3"
	// ibmErrorSnippetLen bounds how much of an unexpected 2xx body is quoted.
	ibmErrorSnippetLen = 200
)

var ibmSpec = Spec{
	Name:        "ibm",
	DisplayName: "IBM Watson Text to Speech",
	Credentials: []CredentialField{
		{
			Key:    "apiKey",
			Prompt: "Enter your IBM Cloud API key for the Text to Speech service: ",
			Help:   "Find it in IBM Cloud > Resource list > your Text to Speech instance > Service credentials",
			Secret: true,
		},
		{
			Key:    "url",
			Prompt: "Enter the service URL (e.g. https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/<id>): ",
			Help:   "It is shown next to the API key in IBM Cloud > Resource list > your Text to Speech instance > Service credentials",
			Secret: false,
		},
	},
	DefaultLang:  "en-US",
	DefaultVoice: "en-US_MichaelV3Voice",
	New: func(creds map[string]string, httpClient *http.Client) (Provider, error) {
		apiKey := strings.TrimSpace(creds["apiKey"])
		if apiKey == "" {
			return nil, fmt.Errorf("ibm: credential %q is required", "apiKey")
		}
		serviceURL := strings.TrimSpace(creds["url"])
		if serviceURL == "" {
			return nil, fmt.Errorf("ibm: credential %q is required", "url")
		}
		if err := validateIBMServiceURL(serviceURL); err != nil {
			return nil, err
		}
		return NewIBMClient(apiKey, serviceURL, httpClient), nil
	},
}

// IBMClient talks to one IBM Watson Text to Speech service instance.
type IBMClient struct {
	// baseURL is the instance service URL, for example
	// https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/<id>,
	// without a trailing slash. API paths are appended to it.
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type ibmSynthesizeRequest struct {
	Text string `json:"text"`
}

type ibmVoice struct {
	Name        string `json:"name"`
	Language    string `json:"language"`
	Gender      string `json:"gender"`
	Description string `json:"description"`
}

type ibmListVoicesResponse struct {
	Voices []ibmVoice `json:"voices"`
}

// NewIBMClient returns an IBM Watson Text to Speech provider for the service
// instance at serviceURL, authenticated with an IBM Cloud API key.
// A nil httpClient selects a default client with a 30 second timeout.
func NewIBMClient(apiKey, serviceURL string, httpClient *http.Client) *IBMClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &IBMClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(serviceURL), "/"),
		apiKey:     apiKey,
		httpClient: client,
	}
}

func (c *IBMClient) Name() string {
	return "ibm"
}

// DefaultVoice returns the service's own default voice for US English. For
// every other language it returns "", and SynthesizeRequest then picks a voice
// for the requested language itself (see resolveVoice).
func (c *IBMClient) DefaultVoice(langCode string) string {
	if strings.EqualFold(langCode, ibmSpec.DefaultLang) {
		return ibmSpec.DefaultVoice
	}
	return ""
}

// SynthesizeRequest performs POST {url}/v1/synthesize and returns raw MP3 bytes.
// req.LanguageCode is not sent: the service has no language parameter and IBM
// voices are language-specific, so the voice name alone selects the language.
// When no voice is given for a language other than en-US, a voice for that
// language is looked up first, because the service would otherwise fall back
// to its en-US default and read the text in the wrong language.
func (c *IBMClient) SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error) {
	payload, err := json.Marshal(ibmSynthesizeRequest{Text: req.Text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	voice, err := c.resolveVoice(ctx, req)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if voice != "" {
		params.Set("voice", voice)
	}
	reqURL, err := c.buildRequestURL(ibmPathSynthesize, params)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", ibmAcceptMP3)

	body, contentType, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty audio in response")
	}
	if !isIBMAudioContentType(contentType) {
		return nil, fmt.Errorf("unexpected non-audio response (Content-Type %q): %s", contentType, ibmBodySnippet(body))
	}
	return body, nil
}

// resolveVoice returns the voice name to send. An explicit VoiceName wins. With
// no voice and no language, or en-US, "" is returned so the voice query is
// omitted and the service default (en-US_MichaelV3Voice) applies. For any other
// language the first voice (sorted by name) the service lists for it is used.
func (c *IBMClient) resolveVoice(ctx context.Context, req SynthRequest) (string, error) {
	if req.VoiceName != "" {
		return req.VoiceName, nil
	}
	lang := strings.TrimSpace(req.LanguageCode)
	if lang == "" || strings.EqualFold(lang, ibmSpec.DefaultLang) {
		return "", nil
	}
	voices, err := c.ListVoices(ctx, lang)
	if err != nil {
		return "", fmt.Errorf("look up a voice for language %q: %w", lang, err)
	}
	if len(voices) == 0 {
		return "", fmt.Errorf("ibm: no voice available for language %q; pass a voice name or list voices for a supported language", lang)
	}
	return voices[0].Name, nil
}

// ListVoices performs GET {url}/v1/voices. The service has no server-side
// language filter, so langCode is applied client side (case-insensitively).
// An empty langCode returns every voice. Results are sorted by name.
func (c *IBMClient) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
	reqURL, err := c.buildRequestURL(ibmPathVoices, nil)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	body, _, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}

	var parsed ibmListVoicesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	voices := make([]Voice, 0, len(parsed.Voices))
	for _, v := range parsed.Voices {
		if langCode != "" && !strings.EqualFold(v.Language, langCode) {
			continue
		}
		voices = append(voices, Voice{
			Name:          v.Name,
			LanguageCodes: []string{v.Language},
			SsmlGender:    strings.ToUpper(strings.TrimSpace(v.Gender)),
		})
	}
	sort.Slice(voices, func(i, j int) bool { return voices[i].Name < voices[j].Name })
	return voices, nil
}

// buildRequestURL appends path (and the encoded params) to the instance URL.
// The instance URL carries its own path (/instances/<id>), so the API path is
// concatenated rather than resolved against it.
func (c *IBMClient) buildRequestURL(path string, params url.Values) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("service url %q must include scheme and host", c.baseURL)
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawPath = ""
	base.RawQuery = params.Encode()
	base.Fragment = ""
	return base.String(), nil
}

// do sends req with HTTP Basic auth (user "apikey", password = API key) and
// returns the body and Content-Type on 2xx, or a formatted API error.
func (c *IBMClient) do(req *http.Request) ([]byte, string, error) {
	req.SetBasicAuth(ibmBasicAuthUser, c.apiKey)
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
		return nil, "", formatIBMAPIError(resp.StatusCode, body)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// formatIBMAPIError formats an IBM error body {"code":N,"error":"...",
// "code_description":"..."}. The shared formatAPIError already reads the
// string "error" field; when that is empty, code_description (the HTTP reason
// phrase, for example "Not Found") is used so the message is never bare.
func formatIBMAPIError(statusCode int, body []byte) error {
	if extractAPIErrorMessage(body) != "" {
		return formatAPIError(statusCode, body)
	}
	var parsed struct {
		CodeDescription string `json:"code_description"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if desc := strings.TrimSpace(parsed.CodeDescription); desc != "" {
			return fmt.Errorf("tts request failed with status %d: %s", statusCode, desc)
		}
	}
	return formatAPIError(statusCode, body)
}

func validateIBMServiceURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("ibm: invalid service url %q: %w", raw, err)
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return fmt.Errorf("ibm: service url %q must start with https:// (or http://) and include a host", raw)
	}
	return nil
}

// isIBMAudioContentType accepts audio/* and application/octet-stream, and an
// absent header. Anything else (JSON, HTML, text) is treated as a non-audio
// response so the caller does not save garbage as an MP3.
func isIBMAudioContentType(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return strings.HasPrefix(mediaType, "audio/") || mediaType == "application/octet-stream"
}

func ibmBodySnippet(body []byte) string {
	if len(body) > ibmErrorSnippetLen {
		body = body[:ibmErrorSnippetLen]
	}
	return strings.TrimSpace(string(body))
}
