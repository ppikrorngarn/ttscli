package tts

// Amazon Polly provider. Verified on 2026-11-06 against:
//   - https://docs.aws.amazon.com/polly/latest/dg/API_SynthesizeSpeech.html
//   - https://docs.aws.amazon.com/polly/latest/dg/API_DescribeVoices.html
//   - https://docs.aws.amazon.com/polly/latest/dg/API_Voice.html
//   - https://docs.aws.amazon.com/polly/latest/dg/voicelist.html
//   - https://docs.aws.amazon.com/general/latest/gr/pol.html (endpoints)
//   - https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-create-signed-request.html
//   - https://github.com/awslabs/aws-c-auth/tree/main/tests/aws-signing-test-suite/v4 (SigV4 vectors)
//   - github.com/aws/aws-sdk-go-v2/service/polly serializers/deserializers
//     (query boolean rendering, X-Amzn-ErrorType sanitizing, body error codes)

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	awsPathSynthesize = "/v1/speech"
	awsPathVoices     = "/v1/voices"
	awsServiceName    = "polly"

	awsEngineNeural   = "neural"
	awsEngineStandard = "standard"

	awsErrorBodyPreview = 200
)

var awsSpec = Spec{
	Name:        "aws",
	DisplayName: "Amazon Polly",
	Credentials: []CredentialField{
		{
			Key:    "accessKeyId",
			Prompt: "Enter your AWS access key ID: ",
			Help:   "Create one in the IAM console under Users > Security credentials; the user needs the polly:SynthesizeSpeech and polly:DescribeVoices permissions",
			Secret: false,
		},
		{
			Key:    "secretAccessKey",
			Prompt: "Enter your AWS secret access key: ",
			Help:   "Shown once when the access key is created in the IAM console; create a new key if you no longer have it",
			Secret: true,
		},
		{
			Key:    "region",
			Prompt: "Enter the AWS region (e.g. us-east-1): ",
			Help:   "Any region where Amazon Polly is available, for example us-east-1 or eu-west-1",
			Secret: false,
		},
	},
	DefaultLang:  "en-US",
	DefaultVoice: "Joanna",
	New: func(creds map[string]string, httpClient *http.Client) (Provider, error) {
		return NewAWSClient(creds["accessKeyId"], creds["secretAccessKey"], creds["region"], httpClient), nil
	},
}

// AWSClient talks to the Amazon Polly REST API, signing every request with
// AWS Signature Version 4.
type AWSClient struct {
	baseURL         string
	accessKeyID     string
	secretAccessKey string
	region          string
	httpClient      *http.Client
	// now supplies the signing clock. Tests pin it for deterministic signatures.
	now func() time.Time
}

type awsSynthesizeRequest struct {
	OutputFormat string `json:"OutputFormat"`
	Text         string `json:"Text"`
	VoiceID      string `json:"VoiceId"`
	LanguageCode string `json:"LanguageCode,omitempty"`
	Engine       string `json:"Engine"`
}

type awsVoice struct {
	ID                      string   `json:"Id"`
	LanguageCode            string   `json:"LanguageCode"`
	LanguageName            string   `json:"LanguageName"`
	Gender                  string   `json:"Gender"`
	Name                    string   `json:"Name"`
	SupportedEngines        []string `json:"SupportedEngines"`
	AdditionalLanguageCodes []string `json:"AdditionalLanguageCodes"`
}

type awsDescribeVoicesResponse struct {
	Voices    []awsVoice `json:"Voices"`
	NextToken string     `json:"NextToken"`
}

// AWSEndpoint returns the regional Amazon Polly endpoint.
func AWSEndpoint(region string) string {
	return fmt.Sprintf("https://polly.%s.amazonaws.com", region)
}

// NewAWSClient returns an Amazon Polly provider authenticated with an IAM
// access key pair. A nil httpClient selects a default client with a 30 second
// timeout.
func NewAWSClient(accessKeyID, secretAccessKey, region string, httpClient *http.Client) *AWSClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &AWSClient{
		baseURL:         AWSEndpoint(region),
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		httpClient:      client,
		now:             time.Now,
	}
}

func (c *AWSClient) Name() string {
	return "aws"
}

// DefaultVoice returns Joanna for US English. Polly has no language-wide
// default voice, so other languages return "" and the caller must pick one
// from ListVoices.
func (c *AWSClient) DefaultVoice(langCode string) string {
	if strings.EqualFold(langCode, awsSpec.DefaultLang) {
		return awsSpec.DefaultVoice
	}
	return ""
}

func (c *AWSClient) SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error) {
	if req.AudioEncoding != AudioEncodingMP3 {
		return nil, fmt.Errorf("unsupported audio encoding %q: Amazon Polly provider only produces %s", req.AudioEncoding, AudioEncodingMP3)
	}

	// Polly's LanguageCode is a case-sensitive enum (en-US, not en-us).
	languageCode := normalizeAWSLanguageCode(req.LanguageCode)

	voiceID := req.VoiceName
	if voiceID == "" {
		voiceID = c.DefaultVoice(languageCode)
	}
	if voiceID == "" {
		return nil, fmt.Errorf("no voice selected for language %q and Amazon Polly requires one: run `ttscli voices --lang %s` and pass --voice <name>", languageCode, languageCode)
	}

	// Neural voices sound better, so try that engine first and fall back to
	// standard when Polly tells us the voice does not support neural.
	audio, err := c.synthesize(ctx, req.Text, languageCode, voiceID, awsEngineNeural)
	if err != nil {
		var apiErr *awsAPIError
		if errors.As(err, &apiErr) && apiErr.isEngineNotSupported() {
			return c.synthesize(ctx, req.Text, languageCode, voiceID, awsEngineStandard)
		}
		return nil, err
	}
	return audio, nil
}

// normalizeAWSLanguageCode spells a BCP-47 style tag the way Polly's
// LanguageCode enum does: lowercase language, uppercase region and variant
// (en-us -> en-US, en-gb-wls -> en-GB-WLS, arb -> arb).
func normalizeAWSLanguageCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	parts := strings.Split(code, "-")
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i])
	}
	return strings.Join(parts, "-")
}

func (c *AWSClient) synthesize(ctx context.Context, text, languageCode, voiceID, engine string) ([]byte, error) {
	payload, err := json.Marshal(awsSynthesizeRequest{
		OutputFormat: "mp3",
		Text:         text,
		VoiceID:      voiceID,
		LanguageCode: languageCode,
		Engine:       engine,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqURL, err := c.buildRequestURL(awsPathSynthesize, nil)
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	body, contentType, err := c.do(req, payload)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty audio stream in response")
	}
	if contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "audio/") {
		return nil, fmt.Errorf("expected audio response, got content type %q: %s", contentType, previewBody(body))
	}
	return body, nil
}

func (c *AWSClient) ListVoices(ctx context.Context, langCode string) ([]Voice, error) {
	langCode = normalizeAWSLanguageCode(langCode)

	var voices []Voice
	nextToken := ""
	for {
		params := url.Values{}
		if langCode != "" {
			params.Set("LanguageCode", langCode)
			// Without this Polly drops bilingual voices whose requested
			// language is only one of their AdditionalLanguageCodes (Aditi
			// for hi-IN). The SDK serializes the boolean as "true".
			params.Set("IncludeAdditionalLanguageCodes", "true")
		}
		if nextToken != "" {
			params.Set("NextToken", nextToken)
		}

		reqURL, err := c.buildRequestURL(awsPathVoices, params)
		if err != nil {
			return nil, fmt.Errorf("build request url: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		body, _, err := c.do(req, nil)
		if err != nil {
			return nil, err
		}

		var parsed awsDescribeVoicesResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		for _, v := range parsed.Voices {
			voice := mapAWSVoice(v)
			if langCode == "" || voiceSpeaks(voice, langCode) {
				voices = append(voices, voice)
			}
		}

		if parsed.NextToken == "" || parsed.NextToken == nextToken {
			break
		}
		nextToken = parsed.NextToken
	}

	sort.Slice(voices, func(i, j int) bool { return voices[i].Name < voices[j].Name })
	return voices, nil
}

func mapAWSVoice(v awsVoice) Voice {
	codes := make([]string, 0, 1+len(v.AdditionalLanguageCodes))
	if v.LanguageCode != "" {
		codes = append(codes, v.LanguageCode)
	}
	codes = append(codes, v.AdditionalLanguageCodes...)
	return Voice{
		Name:          v.ID,
		LanguageCodes: codes,
		SsmlGender:    strings.ToUpper(v.Gender),
	}
}

func voiceSpeaks(v Voice, langCode string) bool {
	for _, code := range v.LanguageCodes {
		if strings.EqualFold(code, langCode) {
			return true
		}
	}
	return false
}

func (c *AWSClient) buildRequestURL(path string, params url.Values) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	ref := &url.URL{Path: path, RawQuery: params.Encode()}
	return baseURL.ResolveReference(ref).String(), nil
}

// do signs req with SigV4 and executes it. payload must be the exact body
// bytes attached to req (nil for a bodyless request). It returns the body and
// the response Content-Type.
func (c *AWSClient) do(req *http.Request, payload []byte) ([]byte, string, error) {
	c.sign(req, payload)

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
		return nil, "", newAWSAPIError(resp.StatusCode, resp.Header.Get("x-amzn-ErrorType"), body)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func previewBody(body []byte) string {
	if len(body) > awsErrorBodyPreview {
		body = body[:awsErrorBodyPreview]
	}
	return strings.TrimSpace(string(body))
}

// awsAPIError is a non-2xx answer from Polly. Polly speaks the AWS REST-JSON
// protocol: the error class travels in the x-amzn-ErrorType header (falling
// back to "code" or "__type" in the body) and the text in a JSON body
// {"message": "..."}.
type awsAPIError struct {
	StatusCode int
	ErrorType  string
	Message    string
}

func newAWSAPIError(statusCode int, errorType string, body []byte) *awsAPIError {
	errorType = sanitizeAWSErrorCode(errorType)
	if errorType == "" {
		errorType = sanitizeAWSErrorCode(extractAWSBodyErrorCode(body))
	}
	return &awsAPIError{
		StatusCode: statusCode,
		ErrorType:  errorType,
		Message:    extractAPIErrorMessage(body),
	}
}

// sanitizeAWSErrorCode reduces an error code to its bare name, the same way
// the AWS SDKs do: "EngineNotSupportedException:http://..." and
// "com.amazonaws.polly#EngineNotSupportedException" both become
// "EngineNotSupportedException".
func sanitizeAWSErrorCode(code string) string {
	if idx := strings.Index(code, ":"); idx >= 0 {
		code = code[:idx]
	}
	if idx := strings.Index(code, "#"); idx >= 0 {
		code = code[idx+1:]
	}
	return strings.TrimSpace(code)
}

// extractAWSBodyErrorCode reads the REST-JSON body error code ("code", then
// "__type") used when the x-amzn-ErrorType header is absent.
func extractAWSBodyErrorCode(body []byte) string {
	var parsed struct {
		Code string `json:"code"`
		Type string `json:"__type"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if parsed.Code != "" {
		return parsed.Code
	}
	return parsed.Type
}

func (e *awsAPIError) Error() string {
	switch {
	case e.Message != "" && e.ErrorType != "":
		return fmt.Sprintf("tts request failed with status %d: %s (%s)", e.StatusCode, e.Message, e.ErrorType)
	case e.Message != "":
		return fmt.Sprintf("tts request failed with status %d: %s", e.StatusCode, e.Message)
	case e.ErrorType != "":
		return fmt.Sprintf("tts request failed with status %d: %s", e.StatusCode, e.ErrorType)
	default:
		return fmt.Sprintf("tts request failed with status %d", e.StatusCode)
	}
}

// isEngineNotSupported reports whether Polly rejected the request because the
// voice does not support the requested engine (EngineNotSupportedException,
// HTTP 400).
func (e *awsAPIError) isEngineNotSupported() bool {
	if e.StatusCode != http.StatusBadRequest {
		return false
	}
	if strings.EqualFold(e.ErrorType, "EngineNotSupportedException") {
		return true
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "engine") &&
		(strings.Contains(msg, "not compatible") || strings.Contains(msg, "not support") || strings.Contains(msg, "does not support"))
}

// ---------------------------------------------------------------------------
// AWS Signature Version 4
//
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-create-signed-request.html
// The signer always signs the host, x-amz-content-sha256 and x-amz-date
// headers, plus content-type when the request carries one (the reference says
// it must be in the canonical headers when present). It signs whatever host
// the request actually targets, so pointing baseURL at a test server produces
// a valid signature for that server.
// ---------------------------------------------------------------------------

const (
	sigV4Algorithm      = "AWS4-HMAC-SHA256"
	sigV4TimeFormat     = "20060102T150405Z"
	sigV4DateFormat     = "20060102"
	sigV4Terminator     = "aws4_request"
	headerAmzDate       = "X-Amz-Date"
	headerAmzContentSHA = "X-Amz-Content-Sha256"
)

// sign adds X-Amz-Date, X-Amz-Content-Sha256 and Authorization to req.
func (c *AWSClient) sign(req *http.Request, payload []byte) {
	now := c.now().UTC()
	amzDate := now.Format(sigV4TimeFormat)
	payloadHash := hexSHA256(payload)

	req.Header.Set(headerAmzDate, amzDate)
	req.Header.Set(headerAmzContentSHA, payloadHash)

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headers := map[string]string{
		"host":                 host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "" {
		headers["content-type"] = contentType
	}

	canonical, signedHeaders := sigV4CanonicalRequest(req.Method, req.URL.Path, req.URL.Query(), headers, payloadHash)
	scope := sigV4CredentialScope(now, c.region, awsServiceName)
	stringToSign := sigV4StringToSign(amzDate, scope, canonical)
	signingKey := sigV4SigningKey(c.secretAccessKey, now, c.region, awsServiceName)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigV4Algorithm, c.accessKeyID, scope, signedHeaders, signature))
}

// sigV4CanonicalRequest builds the canonical request string and the
// semicolon-separated signed header list. headers maps lowercase header names
// to their values; every entry is signed.
func sigV4CanonicalRequest(method, path string, query url.Values, headers map[string]string, payloadHash string) (canonical, signedHeaders string) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)

	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(sigV4TrimHeaderValue(headers[name]))
		canonicalHeaders.WriteString("\n")
	}
	signedHeaders = strings.Join(names, ";")

	canonical = strings.Join([]string{
		method,
		sigV4CanonicalURI(path),
		sigV4CanonicalQuery(query),
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")
	return canonical, signedHeaders
}

func sigV4CanonicalURI(path string) string {
	if path == "" {
		return "/"
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = sigV4URIEncode(seg)
	}
	return strings.Join(segments, "/")
}

func sigV4CanonicalQuery(query url.Values) string {
	type pair struct{ key, value string }
	pairs := make([]pair, 0, len(query))
	for key, values := range query {
		for _, value := range values {
			pairs = append(pairs, pair{sigV4URIEncode(key), sigV4URIEncode(value)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value < pairs[j].value
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.key + "=" + p.value
	}
	return strings.Join(parts, "&")
}

// sigV4URIEncode percent-encodes every byte except the RFC 3986 unreserved
// characters, with uppercase hex digits and spaces as %20.
func sigV4URIEncode(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '.', ch == '_', ch == '~':
			b.WriteByte(ch)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigits[ch>>4])
			b.WriteByte(hexDigits[ch&0x0f])
		}
	}
	return b.String()
}

// sigV4TrimHeaderValue trims the value and collapses runs of spaces.
func sigV4TrimHeaderValue(v string) string {
	return strings.Join(strings.Fields(v), " ")
}

func sigV4CredentialScope(t time.Time, region, service string) string {
	return strings.Join([]string{t.UTC().Format(sigV4DateFormat), region, service, sigV4Terminator}, "/")
}

func sigV4StringToSign(amzDate, scope, canonicalRequest string) string {
	return strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		scope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")
}

func sigV4SigningKey(secret string, t time.Time, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(t.UTC().Format(sigV4DateFormat)))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte(sigV4Terminator))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
