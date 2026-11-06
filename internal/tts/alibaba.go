// Alibaba Cloud Intelligent Speech Interaction (NLS) RESTful text-to-speech
// provider.
//
// Official documentation relied on (checked 2026-11-06):
//   - Speech synthesis RESTful API (endpoint, headers, body, error shape):
//     https://www.alibabacloud.com/help/en/isi/developer-reference/restful-api-3
//   - Obtain an access token (CreateToken overview, Token.Id / Token.ExpireTime):
//     https://www.alibabacloud.com/help/en/isi/getting-started/obtain-an-access-token
//     https://help.aliyun.com/zh/isi/getting-started/obtain-an-access-token
//   - CreateToken over plain HTTP(S), with the signing rules and the worked
//     example used by the tests:
//     https://www.alibabacloud.com/help/en/isi/user-guide/use-http-or-https-to-obtain-an-access-token
//   - Voice list (English and Chinese sites):
//     https://www.alibabacloud.com/help/en/isi/developer-reference/overview-of-speech-synthesis
//     https://help.aliyun.com/zh/isi/developer-reference/overview-of-speech-synthesis
//
// NLS has no list-voices API, so alibabaVoices below is a static table
// transcribed from those pages on 2026-11-06. Locale and gender are derived
// from the "type" and "supported language" columns; see the table comment.
package tts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	alibabaTTSPath            = "/stream/v1/tts"
	alibabaSampleRate         = 16000
	alibabaTokenAction        = "CreateToken"
	alibabaSignatureMethod    = "HMAC-SHA1"
	alibabaSignatureVersion   = "1.0"
	alibabaTimestampLayout    = "2006-01-02T15:04:05Z"
	alibabaTokenExpiryPadding = 60 * time.Second
	alibabaErrorBodyPreview   = 200
)

// alibabaRegion holds the per-region endpoints. The two public regions use
// different meta hosts (note "nls-meta." versus "nlsmeta.") and different
// CreateToken API versions, so they cannot be derived from one template.
type alibabaRegion struct {
	metaBaseURL    string
	gatewayBaseURL string
	apiVersion     string
}

var alibabaRegions = map[string]alibabaRegion{
	"cn-shanghai": {
		metaBaseURL:    "https://nls-meta.cn-shanghai.aliyuncs.com",
		gatewayBaseURL: "https://nls-gateway-cn-shanghai.aliyuncs.com",
		apiVersion:     "2019-02-28",
	},
	"ap-southeast-1": {
		metaBaseURL:    "https://nlsmeta.ap-southeast-1.aliyuncs.com",
		gatewayBaseURL: "https://nls-gateway-ap-southeast-1.aliyuncs.com",
		apiVersion:     "2019-07-17",
	},
}

var alibabaSpec = Spec{
	Name:        "alibaba",
	DisplayName: "Alibaba Cloud Intelligent Speech Interaction",
	Credentials: []CredentialField{
		{
			Key:    "accessKeyId",
			Prompt: "Enter your Alibaba Cloud AccessKey ID: ",
			Help:   "Create one in the RAM console under AccessKey; the RAM user needs the AliyunNLSFullAccess policy or at least nls:CreateToken",
			Secret: false,
		},
		{
			Key:    "accessKeySecret",
			Prompt: "Enter your Alibaba Cloud AccessKey secret: ",
			Help:   "Shown once when the AccessKey is created in the RAM console; create a new AccessKey if you no longer have it",
			Secret: true,
		},
		{
			Key:    "appKey",
			Prompt: "Enter the Intelligent Speech Interaction project Appkey: ",
			Help:   "Open the Intelligent Speech Interaction console, select your project and copy its Appkey",
			Secret: false,
		},
		{
			Key:    "region",
			Prompt: "Enter the region (cn-shanghai or ap-southeast-1): ",
			Help:   "Use cn-shanghai for the Mainland China site and ap-southeast-1 (Singapore) for the international site",
			Secret: false,
		},
	},
	DefaultLang:  "zh-CN",
	DefaultVoice: "xiaoyun",
	New: func(creds map[string]string, httpClient *http.Client) (Provider, error) {
		region := creds["region"]
		if _, ok := alibabaRegions[region]; !ok {
			return nil, fmt.Errorf("unsupported alibaba region %q: use %s", region, strings.Join(alibabaRegionNames(), " or "))
		}
		return NewAlibabaClient(creds["accessKeyId"], creds["accessKeySecret"], creds["appKey"], region, httpClient), nil
	},
}

func alibabaRegionNames() []string {
	names := make([]string, 0, len(alibabaRegions))
	for name := range alibabaRegions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AlibabaClient speaks to the NLS gateway with a short-lived token that it
// obtains lazily from the NLS meta endpoint and reuses for the process
// lifetime (a CLI run is one process).
type AlibabaClient struct {
	metaBaseURL     string
	gatewayBaseURL  string
	apiVersion      string
	region          string
	accessKeyID     string
	accessKeySecret string
	appKey          string
	httpClient      *http.Client

	// now and nonce are injectable for deterministic signing in tests.
	now   func() time.Time
	nonce func() string

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

type alibabaSynthesizeRequest struct {
	AppKey     string `json:"appkey"`
	Text       string `json:"text"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Voice      string `json:"voice"`
}

// alibabaGatewayError is the JSON body the gateway returns instead of audio.
type alibabaGatewayError struct {
	TaskID  string `json:"task_id"`
	Result  string `json:"result"`
	Status  int64  `json:"status"`
	Message string `json:"message"`
}

type alibabaTokenResponse struct {
	Token struct {
		ID         string `json:"Id"`
		ExpireTime int64  `json:"ExpireTime"`
	} `json:"Token"`
	RequestID string `json:"RequestId"`
}

type alibabaMetaError struct {
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
}

// NewAlibabaClient returns an Alibaba Cloud Intelligent Speech Interaction
// provider. region selects the meta and gateway hosts; unknown regions fall
// back to the Mainland China host pattern and API version. A nil httpClient
// selects a default client with a 30 second timeout.
func NewAlibabaClient(accessKeyID, accessKeySecret, appKey, region string, httpClient *http.Client) *AlibabaClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	endpoints, ok := alibabaRegions[region]
	if !ok {
		endpoints = alibabaRegion{
			metaBaseURL:    fmt.Sprintf("https://nls-meta.%s.aliyuncs.com", region),
			gatewayBaseURL: fmt.Sprintf("https://nls-gateway-%s.aliyuncs.com", region),
			apiVersion:     alibabaRegions["cn-shanghai"].apiVersion,
		}
	}
	return &AlibabaClient{
		metaBaseURL:     endpoints.metaBaseURL,
		gatewayBaseURL:  endpoints.gatewayBaseURL,
		apiVersion:      endpoints.apiVersion,
		region:          region,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		appKey:          appKey,
		httpClient:      client,
		now:             time.Now,
		nonce:           alibabaRandomNonce,
	}
}

func (c *AlibabaClient) Name() string {
	return "alibaba"
}

// DefaultVoice returns xiaoyun for zh-CN. For every other language the user
// must pick a voice with --voice; the gateway would otherwise silently fall
// back to xiaoyun, which only speaks Chinese and English.
func (c *AlibabaClient) DefaultVoice(langCode string) string {
	if strings.EqualFold(langCode, alibabaSpec.DefaultLang) {
		return alibabaSpec.DefaultVoice
	}
	return ""
}

func (c *AlibabaClient) SynthesizeRequest(ctx context.Context, req SynthRequest) ([]byte, error) {
	if req.AudioEncoding != "" && req.AudioEncoding != AudioEncodingMP3 {
		return nil, fmt.Errorf("alibaba provider only supports %s output, got %q", AudioEncodingMP3, req.AudioEncoding)
	}
	voice := req.VoiceName
	if voice == "" {
		voice = c.DefaultVoice(req.LanguageCode)
	}
	if voice == "" {
		return nil, fmt.Errorf("alibaba has no default voice for language %q; pass one with --voice (see ttscli voices)", req.LanguageCode)
	}

	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(alibabaSynthesizeRequest{
		AppKey:     c.appKey,
		Text:       req.Text,
		Format:     "mp3",
		SampleRate: alibabaSampleRate,
		Voice:      voice,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqURL, err := alibabaJoinURL(c.gatewayBaseURL, alibabaTTSPath, "")
	if err != nil {
		return nil, fmt.Errorf("build request url: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-NLS-Token", token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	mediaType := alibabaMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "application/json" {
		return nil, alibabaFormatGatewayError(resp.StatusCode, body)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, alibabaFormatGatewayError(resp.StatusCode, body)
	}
	if !strings.HasPrefix(mediaType, "audio/") && mediaType != "application/octet-stream" {
		return nil, fmt.Errorf("unexpected content type %q from tts gateway: %s", resp.Header.Get("Content-Type"), alibabaPreview(body))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty audio response from tts gateway")
	}
	return body, nil
}

// ListVoices returns the static voice table, filtered by locale when langCode
// is not empty. The comparison is case-insensitive.
func (c *AlibabaClient) ListVoices(_ context.Context, langCode string) ([]Voice, error) {
	voices := make([]Voice, 0, len(alibabaVoices))
	for _, v := range alibabaVoices {
		if langCode != "" && !strings.EqualFold(v.locale, langCode) {
			continue
		}
		voices = append(voices, Voice{
			Name:          v.name,
			LanguageCodes: []string{v.locale},
			SsmlGender:    v.gender,
		})
	}
	sort.Slice(voices, func(i, j int) bool { return voices[i].Name < voices[j].Name })
	return voices, nil
}

// getToken returns the cached token, fetching a new one the first time or
// once the cached one is about to expire.
func (c *AlibabaClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Add(alibabaTokenExpiryPadding).Before(c.tokenExpiry) {
		return c.token, nil
	}
	token, expiry, err := c.createToken(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	c.tokenExpiry = expiry
	return token, nil
}

// createToken calls the RPC-style CreateToken action on the meta endpoint.
func (c *AlibabaClient) createToken(ctx context.Context) (string, time.Time, error) {
	params := map[string]string{
		"AccessKeyId":      c.accessKeyID,
		"Action":           alibabaTokenAction,
		"Format":           "JSON",
		"RegionId":         c.region,
		"SignatureMethod":  alibabaSignatureMethod,
		"SignatureNonce":   c.nonce(),
		"SignatureVersion": alibabaSignatureVersion,
		"Timestamp":        c.now().UTC().Format(alibabaTimestampLayout),
		"Version":          c.apiVersion,
	}
	canonical := alibabaCanonicalizedQuery(params)
	signature := alibabaSign(c.accessKeySecret, alibabaStringToSign(http.MethodGet, canonical))
	query := canonical + "&Signature=" + alibabaPercentEncode(signature)

	reqURL, err := alibabaJoinURL(c.metaBaseURL, "/", query)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build token url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create token request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create token: http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create token: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", time.Time{}, fmt.Errorf("create token: %w", alibabaFormatMetaError(resp.StatusCode, body))
	}

	var parsed alibabaTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("create token: unmarshal response: %w", err)
	}
	if parsed.Token.ID == "" {
		return "", time.Time{}, fmt.Errorf("create token: empty Token.Id in response: %s", alibabaPreview(body))
	}
	return parsed.Token.ID, time.Unix(parsed.Token.ExpireTime, 0), nil
}

// alibabaPercentEncode implements the RPC signature encoding: A-Z a-z 0-9 - _ . ~
// are left as-is and every other UTF-8 byte becomes %XY (space is %20, * is
// %2A, ~ stays ~). This is the "encode then fix + * %7E" rule from the docs.
func alibabaPercentEncode(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		default:
			b.WriteByte('%')
			b.WriteByte(upperhex[ch>>4])
			b.WriteByte(upperhex[ch&15])
		}
	}
	return b.String()
}

// alibabaCanonicalizedQuery sorts params by name (byte order), percent-encodes
// names and values and joins them with & as the docs require.
func alibabaCanonicalizedQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, alibabaPercentEncode(k)+"="+alibabaPercentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

// alibabaStringToSign builds HTTPMethod & percentEncode("/") & percentEncode(canonicalQuery).
func alibabaStringToSign(method, canonicalQuery string) string {
	return method + "&" + alibabaPercentEncode("/") + "&" + alibabaPercentEncode(canonicalQuery)
}

// alibabaSign returns base64(HMAC-SHA1(secret + "&", stringToSign)).
func alibabaSign(accessKeySecret, stringToSign string) string {
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func alibabaRandomNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is effectively fatal on supported platforms;
		// fall back to a time-based nonce rather than panicking in a CLI.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func alibabaJoinURL(base, path, rawQuery string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref := &url.URL{Path: path, RawQuery: rawQuery}
	return u.ResolveReference(ref).String(), nil
}

func alibabaMediaType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return strings.ToLower(mediaType)
}

// alibabaFormatGatewayError turns the gateway's JSON error body into
// "tts request failed with status N: <message> (service status S)". A body
// that is not the documented JSON falls back to formatAPIError.
func alibabaFormatGatewayError(statusCode int, body []byte) error {
	var parsed alibabaGatewayError
	if err := json.Unmarshal(body, &parsed); err != nil || (parsed.Message == "" && parsed.Status == 0) {
		return formatAPIError(statusCode, body)
	}
	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = "gateway error"
	}
	detail := fmt.Sprintf("service status %d", parsed.Status)
	if parsed.TaskID != "" {
		detail += ", task_id " + parsed.TaskID
	}
	return fmt.Errorf("tts request failed with status %d: %s (%s)", statusCode, message, detail)
}

// alibabaFormatMetaError handles the {"Code","Message","RequestId"} error body
// of the meta (CreateToken) endpoint.
func alibabaFormatMetaError(statusCode int, body []byte) error {
	var parsed alibabaMetaError
	if err := json.Unmarshal(body, &parsed); err != nil || (parsed.Code == "" && parsed.Message == "") {
		return formatAPIError(statusCode, body)
	}
	message := strings.TrimSpace(parsed.Message)
	if parsed.Code != "" {
		if message != "" {
			message = parsed.Code + ": " + message
		} else {
			message = parsed.Code
		}
	}
	if parsed.RequestID != "" {
		message += " (RequestId " + parsed.RequestID + ")"
	}
	return fmt.Errorf("tts request failed with status %d: %s", statusCode, message)
}

func alibabaPreview(body []byte) string {
	if len(body) > alibabaErrorBodyPreview {
		return string(body[:alibabaErrorBodyPreview]) + "..."
	}
	return string(body)
}

// alibabaVoice is one row of the static voice table.
type alibabaVoice struct {
	name   string
	locale string
	gender string
}

// alibabaVoices is a static snapshot (2026-11-06) of the public voices listed
// on the Intelligent Speech Interaction speech-synthesis API reference, both
// the English and the Chinese site. NLS exposes no list-voices API, so new
// voices need a code change. Conventions:
//   - name is the exact value accepted by the "voice" request field.
//   - locale: Chinese / Chinese-English voices (including Mandarin dialect
//     accents) are zh-CN; "British accent" voices en-GB; "American accent"
//     and "English only" voices en-US; Cantonese voices yue-CN (Kelly, Hong
//     Kong Cantonese, is yue-HK); other languages use their usual BCP-47 tag.
//   - gender follows the docs' "female voice" / "male voice" wording; child
//     voices without a stated sex are NEUTRAL; voices described only by role
//     (announcer, narrator) are left empty.
//
// Digital-human and multi-emotion ("_emo") voices are omitted because they
// are documented for other products and SSML emotion tags respectively.
var alibabaVoices = []alibabaVoice{
	// General scenarios (Chinese and Chinese-English).
	{"xiaoyun", "zh-CN", "FEMALE"},
	{"xiaogang", "zh-CN", "MALE"},
	{"ruoxi", "zh-CN", "FEMALE"},
	{"siqi", "zh-CN", "FEMALE"},
	{"sijia", "zh-CN", "FEMALE"},
	{"sicheng", "zh-CN", "MALE"},
	{"aiqi", "zh-CN", "FEMALE"},
	{"aijia", "zh-CN", "FEMALE"},
	{"aicheng", "zh-CN", "MALE"},
	{"aida", "zh-CN", "MALE"},
	{"ninger", "zh-CN", "FEMALE"},
	{"ruilin", "zh-CN", "FEMALE"},
	{"guijie", "zh-CN", "FEMALE"},
	{"stella", "zh-CN", "FEMALE"},
	{"stanley", "zh-CN", "MALE"},
	{"kenny", "zh-CN", "MALE"},
	{"rosa", "zh-CN", "FEMALE"},
	{"zhitian", "zh-CN", "FEMALE"},
	{"zhiyuan", "zh-CN", "FEMALE"},
	{"zhiyue", "zh-CN", "FEMALE"},
	{"zhida", "zh-CN", "MALE"},
	{"zhistella", "zh-CN", "FEMALE"},
	// Customer-service scenarios.
	{"siyue", "zh-CN", "FEMALE"},
	{"aiya", "zh-CN", "FEMALE"},
	{"aixia", "zh-CN", "FEMALE"},
	{"aimei", "zh-CN", "FEMALE"},
	{"aiyu", "zh-CN", "FEMALE"},
	{"aiyue", "zh-CN", "FEMALE"},
	{"aijing", "zh-CN", "FEMALE"},
	{"xiaomei", "zh-CN", "FEMALE"},
	{"aina", "zh-CN", "FEMALE"},
	{"yina", "zh-CN", "FEMALE"},
	{"sijing", "zh-CN", "FEMALE"},
	{"aishuo", "zh-CN", "MALE"},
	{"zhiya", "zh-CN", "FEMALE"},
	// Child voices.
	{"sitong", "zh-CN", "NEUTRAL"},
	{"xiaobei", "zh-CN", "FEMALE"},
	{"aitong", "zh-CN", "NEUTRAL"},
	{"aiwei", "zh-CN", "FEMALE"},
	{"aibao", "zh-CN", "FEMALE"},
	{"jielidou", "zh-CN", "NEUTRAL"},
	{"mashu", "zh-CN", "MALE"},
	{"yuer", "zh-CN", "FEMALE"},
	// Literature / narration scenarios.
	{"aiyuan", "zh-CN", "FEMALE"},
	{"aiying", "zh-CN", "NEUTRAL"},
	{"aixiang", "zh-CN", "MALE"},
	{"aimo", "zh-CN", "MALE"},
	{"aiye", "zh-CN", "MALE"},
	{"aiting", "zh-CN", "FEMALE"},
	{"aifan", "zh-CN", "FEMALE"},
	{"ainan", "zh-CN", "MALE"},
	{"aihao", "zh-CN", "MALE"},
	{"aiming", "zh-CN", "MALE"},
	{"aixiao", "zh-CN", "FEMALE"},
	{"aichu", "zh-CN", "MALE"},
	{"aiqian", "zh-CN", "FEMALE"},
	{"aishu", "zh-CN", "MALE"},
	{"airu", "zh-CN", "FEMALE"},
	// Ultra-high-definition scenarios.
	{"zhiqi", "zh-CN", "FEMALE"},
	{"zhichu", "zh-CN", "MALE"},
	{"zhixiang", "zh-CN", "MALE"},
	{"zhijia", "zh-CN", "FEMALE"},
	{"zhinan", "zh-CN", "MALE"},
	{"zhiqian", "zh-CN", "FEMALE"},
	{"zhiru", "zh-CN", "FEMALE"},
	{"zhide", "zh-CN", "MALE"},
	{"zhiwei", "zh-CN", "FEMALE"},
	{"zhifei", "zh-CN", ""},
	{"zhilun", "zh-CN", ""},
	// Live-stream scenarios.
	{"xiaoxian", "zh-CN", "FEMALE"},
	{"maoxiaomei", "zh-CN", "FEMALE"},
	{"zhimao", "zh-CN", "FEMALE"},
	{"aifei", "zh-CN", ""},
	{"ailun", "zh-CN", ""},
	{"yaqun", "zh-CN", ""},
	{"qiaowei", "zh-CN", ""},
	{"laotie", "zh-CN", ""},
	{"laomei", "zh-CN", "FEMALE"},
	// Mandarin dialect accents.
	{"xiaoyue", "zh-CN", "FEMALE"},
	{"chuangirl", "zh-CN", "FEMALE"},
	{"qingqing", "zh-CN", "FEMALE"},
	{"zhiqing", "zh-CN", "FEMALE"},
	{"cuijie", "zh-CN", "FEMALE"},
	{"dahu", "zh-CN", "MALE"},
	{"xiaoze", "zh-CN", "MALE"},
	{"aikan", "zh-CN", "MALE"},
	// Cantonese.
	{"shanshan", "yue-CN", "FEMALE"},
	{"jiajia", "yue-CN", "FEMALE"},
	{"taozi", "yue-CN", "FEMALE"},
	{"kelly", "yue-HK", "FEMALE"},
	// English, British accent.
	{"harry", "en-GB", "MALE"},
	{"eric", "en-GB", "MALE"},
	{"emily", "en-GB", "FEMALE"},
	{"luna", "en-GB", "FEMALE"},
	{"luca", "en-GB", "MALE"},
	{"wendy", "en-GB", "FEMALE"},
	{"william", "en-GB", "MALE"},
	{"olivia", "en-GB", "FEMALE"},
	// English, American accent and "English only".
	{"abby", "en-US", "FEMALE"},
	{"andy", "en-US", "MALE"},
	{"annie", "en-US", "FEMALE"},
	{"ava", "en-US", "FEMALE"},
	{"becca", "en-US", "FEMALE"},
	{"betty", "en-US", "FEMALE"},
	{"beth", "en-US", "FEMALE"},
	{"cindy", "en-US", "FEMALE"},
	{"donna", "en-US", "FEMALE"},
	{"eva", "en-US", "FEMALE"},
	{"brian", "en-US", "MALE"},
	{"david", "en-US", "MALE"},
	{"cally", "en-US", "FEMALE"},
	{"lydia", "en-US", "FEMALE"},
	// English with mixed Chinese ("_ecmix").
	{"abby_ecmix", "en-US", "FEMALE"},
	{"annie_ecmix", "en-US", "FEMALE"},
	{"andy_ecmix", "en-US", "MALE"},
	{"ava_ecmix", "en-US", "FEMALE"},
	{"betty_ecmix", "en-US", "FEMALE"},
	{"beth_ecmix", "en-US", "FEMALE"},
	{"brian_ecmix", "en-US", "MALE"},
	{"cindy_ecmix", "en-US", "FEMALE"},
	{"cally_ecmix", "en-US", "FEMALE"},
	{"donna_ecmix", "en-US", "FEMALE"},
	{"david_ecmix", "en-US", "MALE"},
	{"eva_ecmix", "en-US", "FEMALE"},
	// Other languages.
	{"tomoka", "ja-JP", "FEMALE"},
	{"tomoya", "ja-JP", "MALE"},
	{"Kyong", "ko-KR", "FEMALE"},
	{"indah", "id-ID", "FEMALE"},
	{"farah", "ms-MY", "FEMALE"},
	{"tala", "fil-PH", "FEMALE"},
	{"tien", "vi-VN", "FEMALE"},
	{"waan", "th-TH", "FEMALE"},
	{"masha", "ru-RU", "FEMALE"},
	{"camila", "es-ES", "FEMALE"},
	{"perla", "it-IT", "FEMALE"},
	{"clara", "fr-FR", "FEMALE"},
	{"hanna", "de-DE", "FEMALE"},
}
