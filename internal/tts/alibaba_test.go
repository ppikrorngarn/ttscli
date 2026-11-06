package tts

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The documented CreateToken worked example (English "use HTTP or HTTPS to
// obtain an access token" page). The values are fictitious and expired.
const (
	alibabaDocAccessKeyID  = "my_access_key_id"
	alibabaDocSecret       = "my_access_key_secret"
	alibabaDocNonce        = "b924c8c3-6d03-4c5d-ad36-d984d3116788"
	alibabaDocTimestamp    = "2019-04-18T08:32:31Z"
	alibabaDocRegion       = "ap-southeast-1"
	alibabaDocVersion      = "2019-07-17"
	alibabaDocCanonical    = "AccessKeyId=my_access_key_id&Action=CreateToken&Format=JSON&RegionId=ap-southeast-1&SignatureMethod=HMAC-SHA1&SignatureNonce=b924c8c3-6d03-4c5d-ad36-d984d3116788&SignatureVersion=1.0&Timestamp=2019-04-18T08%3A32%3A31Z&Version=2019-07-17"
	alibabaDocStringToSign = "GET&%2F&AccessKeyId%3Dmy_access_key_id%26Action%3DCreateToken%26Format%3DJSON%26RegionId%3Dap-southeast-1%26SignatureMethod%3DHMAC-SHA1%26SignatureNonce%3Db924c8c3-6d03-4c5d-ad36-d984d3116788%26SignatureVersion%3D1.0%26Timestamp%3D2019-04-18T08%253A32%253A31Z%26Version%3D2019-07-17"
	alibabaDocSignature    = "4TdrOuLHXJU+5C6s927Wkq2h+wc="
	alibabaDocSignatureEnc = "4TdrOuLHXJU%2B5C6s927Wkq2h%2Bwc%3D"
)

const alibabaTokenFixture = `{"Token":{"Id":"test-token-id","ExpireTime":4102444800},"RequestId":"2a3b4c5d-6e7f-4a8b-9c0d-1e2f3a4b5c6d"}`

// newAlibabaTestClient builds a client pointed at srv for both hosts, with
// the documented example's fixed timestamp and nonce.
func newAlibabaTestClient(t *testing.T, srv *httptest.Server) *AlibabaClient {
	t.Helper()
	c := NewAlibabaClient(alibabaDocAccessKeyID, alibabaDocSecret, "test-appkey", alibabaDocRegion, srv.Client())
	c.metaBaseURL = srv.URL
	c.gatewayBaseURL = srv.URL
	fixed, err := time.Parse(alibabaTimestampLayout, alibabaDocTimestamp)
	if err != nil {
		t.Fatalf("parse fixture timestamp: %v", err)
	}
	c.now = func() time.Time { return fixed }
	c.nonce = func() string { return alibabaDocNonce }
	return c
}

func alibabaServeToken(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodGet {
		t.Fatalf("token request method: got %s, want GET", r.Method)
	}
	if r.URL.Path != "/" {
		t.Fatalf("token request path: got %q, want /", r.URL.Path)
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, alibabaTokenFixture)
}

func TestAlibabaSignatureMatchesDocumentedVector(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      alibabaDocAccessKeyID,
		"Action":           "CreateToken",
		"Format":           "JSON",
		"RegionId":         alibabaDocRegion,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   alibabaDocNonce,
		"SignatureVersion": "1.0",
		"Timestamp":        alibabaDocTimestamp,
		"Version":          alibabaDocVersion,
		"Signature":        "must-be-excluded",
	}

	canonical := alibabaCanonicalizedQuery(params)
	if canonical != alibabaDocCanonical {
		t.Fatalf("canonicalized query\n got: %s\nwant: %s", canonical, alibabaDocCanonical)
	}

	stringToSign := alibabaStringToSign(http.MethodGet, canonical)
	if stringToSign != alibabaDocStringToSign {
		t.Fatalf("string to sign\n got: %s\nwant: %s", stringToSign, alibabaDocStringToSign)
	}

	// Independent derivation from the documented string-to-sign, so the test
	// checks construction and not just self-agreement.
	mac := hmac.New(sha1.New, []byte(alibabaDocSecret+"&"))
	mac.Write([]byte(alibabaDocStringToSign))
	independent := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if independent != alibabaDocSignature {
		t.Fatalf("independent HMAC does not reproduce the documented signature: got %s", independent)
	}

	if got := alibabaSign(alibabaDocSecret, stringToSign); got != alibabaDocSignature {
		t.Fatalf("alibabaSign: got %s, want %s", got, alibabaDocSignature)
	}
	if got := alibabaPercentEncode(alibabaDocSignature); got != alibabaDocSignatureEnc {
		t.Fatalf("encoded signature: got %s, want %s", got, alibabaDocSignatureEnc)
	}
}

func TestAlibabaPercentEncodeRules(t *testing.T) {
	cases := map[string]string{
		"AZaz09-_.~":    "AZaz09-_.~",
		"a b":           "a%20b",
		"a+b":           "a%2Bb",
		"*":             "%2A",
		"/":             "%2F",
		"=&:":           "%3D%26%3A",
		"中":             "%E4%B8%AD",
		"2019-04-18T08": "2019-04-18T08",
	}
	for in, want := range cases {
		if got := alibabaPercentEncode(in); got != want {
			t.Errorf("alibabaPercentEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlibabaCreateTokenSendsSignedQuery(t *testing.T) {
	var tokenCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&tokenCalls, 1)
			q := r.URL.Query()
			want := map[string]string{
				"AccessKeyId":      alibabaDocAccessKeyID,
				"Action":           "CreateToken",
				"Format":           "JSON",
				"RegionId":         alibabaDocRegion,
				"SignatureMethod":  "HMAC-SHA1",
				"SignatureNonce":   alibabaDocNonce,
				"SignatureVersion": "1.0",
				"Timestamp":        alibabaDocTimestamp,
				"Version":          alibabaDocVersion,
				"Signature":        alibabaDocSignature,
			}
			for k, v := range want {
				if got := q.Get(k); got != v {
					t.Errorf("query %s: got %q, want %q", k, got, v)
				}
			}
			if len(q) != len(want) {
				t.Errorf("query has %d params, want %d: %v", len(q), len(want), q)
			}
			if !strings.Contains(r.URL.RawQuery, "Signature="+alibabaDocSignatureEnc) {
				t.Errorf("raw query does not carry the percent-encoded signature: %s", r.URL.RawQuery)
			}
			alibabaServeToken(t, w, r)
		case alibabaTTSPath:
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("mp3"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	req := SynthRequest{Text: "hi", LanguageCode: "zh-CN", AudioEncoding: AudioEncodingMP3}
	for i := 0; i < 3; i++ {
		if _, err := c.SynthesizeRequest(context.Background(), req); err != nil {
			t.Fatalf("SynthesizeRequest #%d: %v", i, err)
		}
	}
	if n := atomic.LoadInt32(&tokenCalls); n != 1 {
		t.Fatalf("token fetched %d times, want exactly 1 (cached for the process)", n)
	}
}

func TestAlibabaCreateTokenErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"RequestId":"abc-123","Code":"InvalidAccessKeyId.NotFound","Message":"Specified access key is not found."}`)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", LanguageCode: "zh-CN", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{
		"create token",
		"tts request failed with status 404",
		"InvalidAccessKeyId.NotFound: Specified access key is not found.",
		"RequestId abc-123",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestAlibabaCreateTokenEmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"Token":{"Id":"","ExpireTime":0}}`)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	if err == nil || !strings.Contains(err.Error(), "empty Token.Id") {
		t.Fatalf("expected empty Token.Id error, got: %v", err)
	}
}

func TestAlibabaSynthesizeSuccessReturnsBytes(t *testing.T) {
	audio := []byte("ID3\x03\x00fake-mp3-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(audio)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "你好", LanguageCode: "zh-CN", AudioEncoding: AudioEncodingMP3})
	if err != nil {
		t.Fatalf("SynthesizeRequest: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("audio mismatch: got %q", got)
	}
}

func TestAlibabaSynthesizeSendsExactRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/stream/v1/tts" {
			t.Errorf("path: got %s, want /stream/v1/tts", r.URL.Path)
		}
		if got := r.Header.Get("X-NLS-Token"); got != "test-token-id" {
			t.Errorf("X-NLS-Token: got %q, want test-token-id", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type: got %q, want application/json", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		want := map[string]interface{}{
			"appkey":      "test-appkey",
			"text":        "Hello from the test",
			"format":      "mp3",
			"sample_rate": float64(16000),
			"voice":       "harry",
		}
		if len(body) != len(want) {
			t.Errorf("body has %d fields, want %d: %v", len(body), len(want), body)
		}
		for k, v := range want {
			if body[k] != v {
				t.Errorf("body[%s]: got %#v, want %#v", k, body[k], v)
			}
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "Hello from the test", LanguageCode: "en-GB", VoiceName: "harry", AudioEncoding: AudioEncodingMP3})
	if err != nil {
		t.Fatalf("SynthesizeRequest: %v", err)
	}
}

func TestAlibabaSynthesizeGatewayErrorNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"task_id":"8bae3613dfc54ebfa811a17d8a7a1234","result":"","status":40000001,"message":"Gateway:ACCESS_DENIED:The token is invalid"}`)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	want := "tts request failed with status 401: Gateway:ACCESS_DENIED:The token is invalid (service status 40000001, task_id 8bae3613dfc54ebfa811a17d8a7a1234)"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAlibabaSynthesizeGatewayErrorOn200JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `{"task_id":"8f95d0b9b6e948bc98e8d0ce64b01234","result":"","status":40000000,"message":"Gateway:CLIENT_ERROR:in post data, json format illegal"}`)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	if err == nil {
		t.Fatal("expected error for JSON body on 200")
	}
	for _, want := range []string{"tts request failed with status 200", "Gateway:CLIENT_ERROR:in post data, json format illegal", "40000000"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestAlibabaSynthesizeNon2xxPlainBodyFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	if err == nil || err.Error() != "tts request failed with status 502" {
		t.Fatalf("expected status-only error, got: %v", err)
	}
}

func TestAlibabaSynthesizeUnexpectedContentTypeIncludesBodyPreview(t *testing.T) {
	long := strings.Repeat("x", 500)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			alibabaServeToken(t, w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, long)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unexpected content type "text/html"`) {
		t.Errorf("error should name the content type: %s", msg)
	}
	if !strings.Contains(msg, strings.Repeat("x", 200)+"...") {
		t.Errorf("error should include the first 200 bytes of the body: %s", msg)
	}
	if strings.Contains(msg, strings.Repeat("x", 201)) {
		t.Errorf("error should truncate the body preview: %s", msg)
	}
}

func TestAlibabaSynthesizeRequiresVoiceForNonDefaultLanguage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", LanguageCode: "en-US", AudioEncoding: AudioEncodingMP3})
	if err == nil || !strings.Contains(err.Error(), "--voice") {
		t.Fatalf("expected an error asking for --voice, got: %v", err)
	}
}

func TestAlibabaSynthesizeRejectsNonMP3Encoding(t *testing.T) {
	c := NewAlibabaClient("id", "secret", "appkey", "cn-shanghai", nil)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: "LINEAR16"})
	if err == nil || !strings.Contains(err.Error(), "MP3") {
		t.Fatalf("expected unsupported encoding error, got: %v", err)
	}
}

func TestAlibabaListVoicesFiltersCaseInsensitively(t *testing.T) {
	c := NewAlibabaClient("id", "secret", "appkey", "cn-shanghai", nil)

	voices, err := c.ListVoices(context.Background(), "EN-gb")
	if err != nil {
		t.Fatalf("ListVoices: %v", err)
	}
	if len(voices) == 0 {
		t.Fatal("expected en-GB voices")
	}
	names := make([]string, 0, len(voices))
	for _, v := range voices {
		if len(v.LanguageCodes) != 1 || v.LanguageCodes[0] != "en-GB" {
			t.Errorf("voice %s has language codes %v, want [en-GB]", v.Name, v.LanguageCodes)
		}
		if v.SsmlGender != "MALE" && v.SsmlGender != "FEMALE" {
			t.Errorf("voice %s has gender %q", v.Name, v.SsmlGender)
		}
		names = append(names, v.Name)
	}
	for _, want := range []string{"harry", "eric", "emily", "luna", "luca", "wendy", "william", "olivia"} {
		if !containsString(names, want) {
			t.Errorf("en-GB voices missing %q: %v", want, names)
		}
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("voices not sorted by name: %v", names)
		}
	}
}

func TestAlibabaListVoicesEmptyLanguageReturnsAll(t *testing.T) {
	c := NewAlibabaClient("id", "secret", "appkey", "cn-shanghai", nil)
	voices, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices: %v", err)
	}
	if len(voices) != len(alibabaVoices) {
		t.Fatalf("got %d voices, want all %d", len(voices), len(alibabaVoices))
	}
	seen := map[string]bool{}
	for i, v := range voices {
		if seen[v.Name] {
			t.Errorf("duplicate voice %q", v.Name)
		}
		seen[v.Name] = true
		if i > 0 && voices[i-1].Name > v.Name {
			t.Errorf("voices not sorted at %d: %q > %q", i, voices[i-1].Name, v.Name)
		}
		switch v.SsmlGender {
		case "", "MALE", "FEMALE", "NEUTRAL":
		default:
			t.Errorf("voice %s has invalid gender %q", v.Name, v.SsmlGender)
		}
	}
	for _, want := range []string{"xiaoyun", "xiaogang", "ruoxi", "siqi", "sijia", "sicheng", "aiqi", "aijia", "aicheng", "aida",
		"harry", "abby", "andy", "eric", "emily", "luna", "luca", "wendy", "william", "olivia"} {
		if !seen[want] {
			t.Errorf("voice table missing %q", want)
		}
	}
	zh, _ := c.ListVoices(context.Background(), "zh-cn")
	if len(zh) == 0 || len(zh) >= len(voices) {
		t.Fatalf("zh-CN filter returned %d of %d voices", len(zh), len(voices))
	}
}

func TestAlibabaDefaultVoice(t *testing.T) {
	c := NewAlibabaClient("id", "secret", "appkey", "cn-shanghai", nil)
	if got := c.DefaultVoice("zh-CN"); got != "xiaoyun" {
		t.Fatalf("DefaultVoice(zh-CN) = %q", got)
	}
	if got := c.DefaultVoice("ZH-cn"); got != "xiaoyun" {
		t.Fatalf("DefaultVoice(ZH-cn) = %q, want case-insensitive match", got)
	}
	for _, lang := range []string{"en-US", "en-GB", "ja-JP", ""} {
		if got := c.DefaultVoice(lang); got != "" {
			t.Errorf("DefaultVoice(%q) = %q, want empty", lang, got)
		}
	}
	if alibabaSpec.DefaultVoice != "xiaoyun" || alibabaSpec.DefaultLang != "zh-CN" {
		t.Fatalf("spec defaults: %q / %q", alibabaSpec.DefaultLang, alibabaSpec.DefaultVoice)
	}
}

func TestAlibabaRegionEndpoints(t *testing.T) {
	sh := NewAlibabaClient("id", "secret", "appkey", "cn-shanghai", nil)
	if sh.metaBaseURL != "https://nls-meta.cn-shanghai.aliyuncs.com" || sh.gatewayBaseURL != "https://nls-gateway-cn-shanghai.aliyuncs.com" || sh.apiVersion != "2019-02-28" {
		t.Fatalf("cn-shanghai endpoints: %s %s %s", sh.metaBaseURL, sh.gatewayBaseURL, sh.apiVersion)
	}
	sg := NewAlibabaClient("id", "secret", "appkey", "ap-southeast-1", nil)
	if sg.metaBaseURL != "https://nlsmeta.ap-southeast-1.aliyuncs.com" || sg.gatewayBaseURL != "https://nls-gateway-ap-southeast-1.aliyuncs.com" || sg.apiVersion != "2019-07-17" {
		t.Fatalf("ap-southeast-1 endpoints: %s %s %s", sg.metaBaseURL, sg.gatewayBaseURL, sg.apiVersion)
	}
	if sh.httpClient == nil || sh.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("nil httpClient should yield the default client with timeout %v", defaultHTTPTimeout)
	}
}

func TestAlibabaSpecNewBuildsClientAndConsumesEveryCredential(t *testing.T) {
	if alibabaSpec.Name != "alibaba" {
		t.Fatalf("spec name %q", alibabaSpec.Name)
	}
	wantKeys := []string{"accessKeyId", "accessKeySecret", "appKey", "region"}
	if len(alibabaSpec.Credentials) != len(wantKeys) {
		t.Fatalf("spec has %d credentials, want %d", len(alibabaSpec.Credentials), len(wantKeys))
	}
	for i, field := range alibabaSpec.Credentials {
		if field.Key != wantKeys[i] {
			t.Errorf("credential %d key %q, want %q", i, field.Key, wantKeys[i])
		}
		if !strings.HasSuffix(field.Prompt, ": ") {
			t.Errorf("credential %q prompt must end with \": \": %q", field.Key, field.Prompt)
		}
		if field.Help == "" {
			t.Errorf("credential %q has no Help", field.Key)
		}
	}
	if !alibabaSpec.Credentials[1].Secret {
		t.Error("accessKeySecret must be Secret")
	}
	for _, i := range []int{0, 2, 3} {
		if alibabaSpec.Credentials[i].Secret {
			t.Errorf("credential %q must not be Secret", alibabaSpec.Credentials[i].Key)
		}
	}

	// Every declared key must be consumed: build with a distinct value per key
	// and check each one landed on the client.
	creds := map[string]string{}
	for _, field := range alibabaSpec.Credentials {
		creds[field.Key] = "value-for-" + field.Key
	}
	creds["region"] = "ap-southeast-1"

	hc := &http.Client{}
	p, err := alibabaSpec.New(creds, hc)
	if err != nil {
		t.Fatalf("Spec.New: %v", err)
	}
	c, ok := p.(*AlibabaClient)
	if !ok {
		t.Fatalf("Spec.New returned %T", p)
	}
	consumed := map[string]string{
		"accessKeyId":     c.accessKeyID,
		"accessKeySecret": c.accessKeySecret,
		"appKey":          c.appKey,
		"region":          c.region,
	}
	for _, field := range alibabaSpec.Credentials {
		if consumed[field.Key] != creds[field.Key] {
			t.Errorf("credential %q not consumed by New: client has %q", field.Key, consumed[field.Key])
		}
	}
	if c.httpClient != hc {
		t.Error("Spec.New did not pass the http client through")
	}
	if p.Name() != "alibaba" {
		t.Errorf("Name() = %q", p.Name())
	}

	// The constructed client must actually work against a server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if got := r.URL.Query().Get("AccessKeyId"); got != "value-for-accessKeyId" {
				t.Errorf("AccessKeyId: %q", got)
			}
			if got := r.URL.Query().Get("RegionId"); got != "ap-southeast-1" {
				t.Errorf("RegionId: %q", got)
			}
			alibabaServeToken(t, w, r)
			return
		}
		var body alibabaSynthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.AppKey != "value-for-appKey" {
			t.Errorf("appkey: %q", body.AppKey)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c.metaBaseURL = srv.URL
	c.gatewayBaseURL = srv.URL
	c.httpClient = srv.Client()
	got, err := p.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", LanguageCode: "zh-CN", AudioEncoding: AudioEncodingMP3})
	if err != nil || string(got) != "ok" {
		t.Fatalf("SynthesizeRequest via Spec.New client: %q, %v", got, err)
	}
}

func TestAlibabaSpecNewRejectsUnknownRegion(t *testing.T) {
	_, err := alibabaSpec.New(map[string]string{
		"accessKeyId": "a", "accessKeySecret": "b", "appKey": "c", "region": "us-west-1",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "us-west-1") || !strings.Contains(err.Error(), "cn-shanghai") {
		t.Fatalf("expected unsupported region error naming the valid regions, got: %v", err)
	}
}

func TestAlibabaTokenRefreshesWhenExpired(t *testing.T) {
	var tokenCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			n := atomic.AddInt32(&tokenCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			// Expires 30 seconds after the fixed clock, inside the refresh padding.
			fmt.Fprintf(w, `{"Token":{"Id":"tok-%d","ExpireTime":1555576381}}`, n)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newAlibabaTestClient(t, srv)
	req := SynthRequest{Text: "hi", VoiceName: "xiaoyun", AudioEncoding: AudioEncodingMP3}
	for i := 0; i < 2; i++ {
		if _, err := c.SynthesizeRequest(context.Background(), req); err != nil {
			t.Fatalf("SynthesizeRequest #%d: %v", i, err)
		}
	}
	if n := atomic.LoadInt32(&tokenCalls); n != 2 {
		t.Fatalf("token fetched %d times, want 2 for a nearly expired token", n)
	}
}

func TestAlibabaRandomNonceIsUnique(t *testing.T) {
	a, b := alibabaRandomNonce(), alibabaRandomNonce()
	if a == "" || a == b {
		t.Fatalf("nonces must be non-empty and unique: %q %q", a, b)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
