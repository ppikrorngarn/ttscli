package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Official SigV4 test-suite credentials (aws-c-auth aws-signing-test-suite).
const (
	sigV4TestAccessKey = "AKIDEXAMPLE"
	sigV4TestSecretKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
)

var sigV4TestTime = time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)

// awsDescribeVoicesFixture mirrors the DescribeVoices response shape from the
// API reference, including a bilingual voice with AdditionalLanguageCodes.
const awsDescribeVoicesFixture = `{
  "Voices": [
    {
      "Gender": "Female",
      "Id": "Joanna",
      "LanguageCode": "en-US",
      "LanguageName": "US English",
      "Name": "Joanna",
      "SupportedEngines": ["neural", "standard"]
    },
    {
      "Gender": "Male",
      "Id": "Matthew",
      "LanguageCode": "en-US",
      "LanguageName": "US English",
      "Name": "Matthew",
      "SupportedEngines": ["generative", "neural", "standard"]
    },
    {
      "AdditionalLanguageCodes": ["hi-IN"],
      "Gender": "Female",
      "Id": "Aditi",
      "LanguageCode": "en-IN",
      "LanguageName": "Indian English",
      "Name": "Aditi",
      "SupportedEngines": ["standard"]
    },
    {
      "Gender": "Male",
      "Id": "Hans",
      "LanguageCode": "de-DE",
      "LanguageName": "German",
      "Name": "Hans",
      "SupportedEngines": ["standard"]
    }
  ]
}`

func newTestAWSClient(t *testing.T, srv *httptest.Server) *AWSClient {
	t.Helper()
	c := NewAWSClient(sigV4TestAccessKey, sigV4TestSecretKey, "us-east-1", srv.Client())
	c.baseURL = srv.URL
	c.now = func() time.Time { return sigV4TestTime }
	return c
}

func writeAWSError(w http.ResponseWriter, status int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	if errorType != "" {
		w.Header().Set("x-amzn-ErrorType", errorType)
	}
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"message":%q}`, message)
}

func TestAWSSynthesizeSuccess(t *testing.T) {
	audio := []byte("ID3-fake-mp3-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != awsPathSynthesize {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(audio)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("unexpected audio payload: %q", string(got))
	}
}

func TestAWSSynthesizeRequestShape(t *testing.T) {
	var (
		gotBody    awsSynthesizeRequest
		gotRaw     []byte
		gotHeaders http.Header
		gotHost    string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotRaw = raw
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotHeaders = r.Header.Clone()
		gotHost = r.Host
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello world", LanguageCode: "en-GB", VoiceName: "Amy", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}

	want := awsSynthesizeRequest{OutputFormat: "mp3", Text: "hello world", VoiceID: "Amy", LanguageCode: "en-GB", Engine: "neural"}
	if gotBody != want {
		t.Fatalf("unexpected body: %#v, want %#v", gotBody, want)
	}
	// Exact key names matter to Polly; make sure the JSON uses them.
	for _, key := range []string{`"OutputFormat"`, `"Text"`, `"VoiceId"`, `"LanguageCode"`, `"Engine"`} {
		if !strings.Contains(string(gotRaw), key) {
			t.Fatalf("body is missing key %s: %s", key, gotRaw)
		}
	}

	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if got := gotHeaders.Get("X-Amz-Date"); got != "20150830T123600Z" {
		t.Fatalf("unexpected X-Amz-Date: %q", got)
	}
	if got := gotHeaders.Get("X-Amz-Content-Sha256"); got != hexSHA256(gotRaw) {
		t.Fatalf("X-Amz-Content-Sha256 %q does not match body hash %q", got, hexSHA256(gotRaw))
	}

	auth := gotHeaders.Get("Authorization")
	prefix := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/polly/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="
	if !strings.HasPrefix(auth, prefix) {
		t.Fatalf("unexpected Authorization header: %q", auth)
	}
	signature := strings.TrimPrefix(auth, prefix)
	if len(signature) != 64 || strings.Trim(signature, "0123456789abcdef") != "" {
		t.Fatalf("signature is not 64 lowercase hex chars: %q", signature)
	}

	// Recompute the signature server-side from the received request and the
	// shared secret; it must match what the client sent.
	payloadHash := hexSHA256(gotRaw)
	canonical, _ := sigV4CanonicalRequest(http.MethodPost, awsPathSynthesize, url.Values{}, map[string]string{
		"host":                 gotHost,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           "20150830T123600Z",
	}, payloadHash)
	scope := sigV4CredentialScope(sigV4TestTime, "us-east-1", "polly")
	wantSig := hexHMAC(sigV4SigningKey(sigV4TestSecretKey, sigV4TestTime, "us-east-1", "polly"), sigV4StringToSign("20150830T123600Z", scope, canonical))
	if signature != wantSig {
		t.Fatalf("signature mismatch: got %s want %s", signature, wantSig)
	}
}

func TestAWSSynthesizeErrorStatusIncludesMessageAndType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAWSError(w, http.StatusForbidden, "UnrecognizedClientException:http://internal.amazon.com/coral/com.amazon.coral.service/", "The security token included in the request is invalid.")
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	want := "tts request failed with status 403: The security token included in the request is invalid. (UnrecognizedClientException)"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAWSSynthesizeErrorStatusWithoutJSONBody(t *testing.T) {
	const secret = "aws-sentinel-secret-98765"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, secret)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || err.Error() != "tts request failed with status 500" {
		t.Fatalf("expected status-only error, got: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("response body leaked into error: %v", err)
	}
}

func TestAWSSynthesizeFallsBackToStandardEngine(t *testing.T) {
	var engines []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body awsSynthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		engines = append(engines, body.Engine)
		if body.Engine == "neural" {
			writeAWSError(w, http.StatusBadRequest, "EngineNotSupportedException", "This engine is not compatible with the voice that you have designated. Choose a new voice that is compatible with the engine or change the engine and restart the operation.")
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("standard-mp3"))
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hallo", LanguageCode: "de-DE", VoiceName: "Hans", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if string(got) != "standard-mp3" {
		t.Fatalf("unexpected audio: %q", got)
	}
	if len(engines) != 2 || engines[0] != "neural" || engines[1] != "standard" {
		t.Fatalf("unexpected engine sequence: %v", engines)
	}
}

func TestAWSSynthesizeFallsBackOnMessageOnly(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeAWSError(w, http.StatusBadRequest, "", "The voice Hans does not support the neural engine")
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	if _, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hallo", LanguageCode: "de-DE", VoiceName: "Hans", AudioEncoding: AudioEncodingMP3,
	}); err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestAWSSynthesizeDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeAWSError(w, http.StatusBadRequest, "TextLengthExceededException", "Text length exceeds maximum allowed length.")
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "status 400") || !strings.Contains(err.Error(), "TextLengthExceededException") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected a single call, got %d", calls)
	}
}

func TestAWSSynthesizeUsesDefaultVoiceForDefaultLang(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body awsSynthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.VoiceID != "Joanna" {
			t.Fatalf("expected default voice Joanna, got %q", body.VoiceID)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	if _, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", AudioEncoding: AudioEncodingMP3,
	}); err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestAWSSynthesizeRequiresVoiceForOtherLang(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request expected")
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hallo", LanguageCode: "de-DE", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "ttscli voices --lang de-DE") || !strings.Contains(err.Error(), "--voice") {
		t.Fatalf("expected guidance error, got: %v", err)
	}
}

func TestAWSSynthesizeRejectsNonAudioBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"unexpected":"json body"}`)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), `application/json`) || !strings.Contains(err.Error(), `{"unexpected":"json body"}`) {
		t.Fatalf("expected non-audio error with body preview, got: %v", err)
	}
}

func TestAWSSynthesizeRejectsEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "empty audio stream") {
		t.Fatalf("expected empty body error, got: %v", err)
	}
}

func TestAWSSynthesizeRejectsUnsupportedEncoding(t *testing.T) {
	c := NewAWSClient("a", "b", "us-east-1", nil)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "Joanna", AudioEncoding: "OGG_OPUS",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported audio encoding") {
		t.Fatalf("expected unsupported encoding error, got: %v", err)
	}
}

func TestAWSListVoicesParsesAndFiltersCaseInsensitively(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != awsPathVoices {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("LanguageCode"); got != "EN-us" {
			t.Fatalf("unexpected LanguageCode query: %q", got)
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, awsDescribeVoicesFixture)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "EN-us")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 2 {
		t.Fatalf("expected 2 en-US voices, got %#v", voices)
	}
	if voices[0].Name != "Joanna" || voices[1].Name != "Matthew" {
		t.Fatalf("voices not sorted by name: %#v", voices)
	}
	if voices[0].SsmlGender != "FEMALE" || voices[1].SsmlGender != "MALE" {
		t.Fatalf("unexpected genders: %#v", voices)
	}
	if len(voices[0].LanguageCodes) != 1 || voices[0].LanguageCodes[0] != "en-US" {
		t.Fatalf("unexpected language codes: %#v", voices[0].LanguageCodes)
	}
}

func TestAWSListVoicesMatchesAdditionalLanguageCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, awsDescribeVoicesFixture)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "hi-in")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 1 || voices[0].Name != "Aditi" {
		t.Fatalf("expected Aditi via AdditionalLanguageCodes, got %#v", voices)
	}
	if strings.Join(voices[0].LanguageCodes, ",") != "en-IN,hi-IN" {
		t.Fatalf("unexpected language codes: %#v", voices[0].LanguageCodes)
	}
}

func TestAWSListVoicesEmptyLangReturnsAllAndOmitsParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.URL.Query()["LanguageCode"]; present {
			t.Fatalf("expected no LanguageCode query, got %q", r.URL.RawQuery)
		}
		fmt.Fprint(w, awsDescribeVoicesFixture)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	names := make([]string, len(voices))
	for i, v := range voices {
		names[i] = v.Name
	}
	if strings.Join(names, ",") != "Aditi,Hans,Joanna,Matthew" {
		t.Fatalf("unexpected voices: %v", names)
	}
}

func TestAWSListVoicesPaginates(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("NextToken")
		tokens = append(tokens, token)
		w.Header().Set("Content-Type", "application/json")
		switch token {
		case "":
			fmt.Fprint(w, `{"Voices":[{"Id":"Zhiyu","LanguageCode":"cmn-CN","Gender":"Female","Name":"Zhiyu","SupportedEngines":["neural","standard"]}],"NextToken":"page-2"}`)
		case "page-2":
			fmt.Fprint(w, `{"Voices":[{"Id":"Hiujin","LanguageCode":"yue-CN","Gender":"Female","Name":"Hiujin","SupportedEngines":["neural"]}]}`)
		default:
			t.Fatalf("unexpected NextToken %q", token)
		}
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(tokens) != 2 || tokens[0] != "" || tokens[1] != "page-2" {
		t.Fatalf("unexpected pagination sequence: %q", tokens)
	}
	if len(voices) != 2 || voices[0].Name != "Hiujin" || voices[1].Name != "Zhiyu" {
		t.Fatalf("unexpected voices: %#v", voices)
	}
}

func TestAWSListVoicesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "1 validation error detected")
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.ListVoices(context.Background(), "xx-XX")
	want := "tts request failed with status 400: 1 validation error detected (ValidationException)"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAWSListVoicesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{invalid-json`)
	}))
	defer srv.Close()

	c := newTestAWSClient(t, srv)
	_, err := c.ListVoices(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestAWSDefaultVoice(t *testing.T) {
	c := NewAWSClient("a", "b", "us-east-1", nil)
	cases := map[string]string{
		"en-US": "Joanna",
		"en-us": "Joanna",
		"EN-US": "Joanna",
		"en-GB": "",
		"de-DE": "",
		"":      "",
	}
	for lang, want := range cases {
		if got := c.DefaultVoice(lang); got != want {
			t.Fatalf("DefaultVoice(%q) = %q, want %q", lang, got, want)
		}
	}
}

func TestAWSNewClientDefaults(t *testing.T) {
	c := NewAWSClient("a", "b", "eu-west-1", nil)
	if c.httpClient == nil || c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("expected default http client with %v timeout, got %#v", defaultHTTPTimeout, c.httpClient)
	}
	if c.baseURL != "https://polly.eu-west-1.amazonaws.com" {
		t.Fatalf("unexpected base URL: %q", c.baseURL)
	}
	if c.Name() != "aws" {
		t.Fatalf("unexpected name: %q", c.Name())
	}
	if c.now == nil {
		t.Fatal("clock must be set")
	}
}

func TestAWSSpecShape(t *testing.T) {
	if awsSpec.Name != "aws" || awsSpec.DisplayName != "Amazon Polly" {
		t.Fatalf("unexpected spec identity: %#v", awsSpec)
	}
	if awsSpec.DefaultLang != "en-US" || awsSpec.DefaultVoice != "Joanna" {
		t.Fatalf("unexpected spec defaults: %q %q", awsSpec.DefaultLang, awsSpec.DefaultVoice)
	}
	wantKeys := []string{"accessKeyId", "secretAccessKey", "region"}
	if len(awsSpec.Credentials) != len(wantKeys) {
		t.Fatalf("expected %d credentials, got %d", len(wantKeys), len(awsSpec.Credentials))
	}
	for i, field := range awsSpec.Credentials {
		if field.Key != wantKeys[i] {
			t.Fatalf("credential %d key = %q, want %q", i, field.Key, wantKeys[i])
		}
		if !strings.HasSuffix(field.Prompt, ": ") {
			t.Fatalf("credential %q prompt must end with ': ', got %q", field.Key, field.Prompt)
		}
		if field.Help == "" {
			t.Fatalf("credential %q has no help text", field.Key)
		}
		if field.Secret != (field.Key == "secretAccessKey") {
			t.Fatalf("credential %q has unexpected Secret=%v", field.Key, field.Secret)
		}
	}
}

func TestAWSSpecNewBuildsWorkingClient(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"Voices":[]}`)
	}))
	defer srv.Close()

	creds := map[string]string{"accessKeyId": "AKIDTEST", "secretAccessKey": "secret", "region": "ap-southeast-1"}
	p, err := awsSpec.New(creds, srv.Client())
	if err != nil {
		t.Fatalf("Spec.New returned error: %v", err)
	}
	c, ok := p.(*AWSClient)
	if !ok {
		t.Fatalf("Spec.New returned %T, want *AWSClient", p)
	}
	if c.baseURL != "https://polly.ap-southeast-1.amazonaws.com" {
		t.Fatalf("unexpected base URL: %q", c.baseURL)
	}
	c.baseURL = srv.URL

	if _, err := p.ListVoices(context.Background(), ""); err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if !strings.Contains(gotAuth, "Credential=AKIDTEST/") || !strings.Contains(gotAuth, "/ap-southeast-1/polly/aws4_request") {
		t.Fatalf("Authorization did not use the supplied credentials: %q", gotAuth)
	}
}

// Every declared credential must change the outcome of signing: that proves
// New reads each key the Spec advertises.
func TestAWSSpecCredentialsAreAllConsumed(t *testing.T) {
	base := map[string]string{}
	for i, field := range awsSpec.Credentials {
		base[field.Key] = fmt.Sprintf("value-%d", i)
	}
	signWith := func(creds map[string]string) string {
		p, err := awsSpec.New(creds, &http.Client{})
		if err != nil {
			t.Fatalf("Spec.New returned error: %v", err)
		}
		c := p.(*AWSClient)
		c.now = func() time.Time { return sigV4TestTime }
		req, err := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/v1/voices", nil)
		if err != nil {
			t.Fatal(err)
		}
		c.sign(req, nil)
		return req.Header.Get("Authorization")
	}

	baseline := signWith(base)
	for _, field := range awsSpec.Credentials {
		mutated := make(map[string]string, len(base))
		for k, v := range base {
			mutated[k] = v
		}
		mutated[field.Key] = "changed"
		if signWith(mutated) == baseline {
			t.Fatalf("credential %q is declared but not consumed by New", field.Key)
		}
	}
}

// Vector: aws-signing-test-suite/v4/get-vanilla (GET / on example.amazonaws.com,
// 2015-08-30T12:36:00Z, region us-east-1, service "service").
func TestSigV4GetVanillaVector(t *testing.T) {
	canonical, signed := sigV4CanonicalRequest(http.MethodGet, "/", url.Values{}, map[string]string{
		"host":       "example.amazonaws.com",
		"x-amz-date": "20150830T123600Z",
	}, hexSHA256(nil))

	wantCanonical := "GET\n/\n\nhost:example.amazonaws.com\nx-amz-date:20150830T123600Z\n\nhost;x-amz-date\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if canonical != wantCanonical {
		t.Fatalf("canonical request mismatch:\n%q\nwant\n%q", canonical, wantCanonical)
	}
	if signed != "host;x-amz-date" {
		t.Fatalf("unexpected signed headers: %q", signed)
	}

	scope := sigV4CredentialScope(sigV4TestTime, "us-east-1", "service")
	sts := sigV4StringToSign("20150830T123600Z", scope, canonical)
	wantSTS := "AWS4-HMAC-SHA256\n20150830T123600Z\n20150830/us-east-1/service/aws4_request\nbb579772317eb040ac9ed261061d46c1f17a8133879d6129b6e1c25292927e63"
	if sts != wantSTS {
		t.Fatalf("string to sign mismatch:\n%q\nwant\n%q", sts, wantSTS)
	}

	got := hexHMAC(sigV4SigningKey(sigV4TestSecretKey, sigV4TestTime, "us-east-1", "service"), sts)
	if got != "5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31" {
		t.Fatalf("unexpected signature: %s", got)
	}
}

// Vector: aws-signing-test-suite/v4/post-x-www-form-urlencoded, which signs a
// payload hash and content headers.
func TestSigV4PostFormVector(t *testing.T) {
	payload := []byte("Param1=value1")
	payloadHash := hexSHA256(payload)
	if payloadHash != "9095672bbd1f56dfc5b65f3e153adc8731a4a654192329106275f4c7b24d0b6e" {
		t.Fatalf("unexpected payload hash: %s", payloadHash)
	}

	canonical, signed := sigV4CanonicalRequest(http.MethodPost, "/", url.Values{}, map[string]string{
		"content-length":       "13",
		"content-type":         "application/x-www-form-urlencoded",
		"host":                 "example.amazonaws.com",
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           "20150830T123600Z",
	}, payloadHash)
	if signed != "content-length;content-type;host;x-amz-content-sha256;x-amz-date" {
		t.Fatalf("unexpected signed headers: %q", signed)
	}

	scope := sigV4CredentialScope(sigV4TestTime, "us-east-1", "service")
	sts := sigV4StringToSign("20150830T123600Z", scope, canonical)
	got := hexHMAC(sigV4SigningKey(sigV4TestSecretKey, sigV4TestTime, "us-east-1", "service"), sts)
	if got != "d3875051da38690788ef43de4db0d8f280229d82040bfac253562e56c3f20e0b" {
		t.Fatalf("unexpected signature: %s", got)
	}
}

func TestSigV4SignIsDeterministicAndPayloadSensitive(t *testing.T) {
	c := NewAWSClient(sigV4TestAccessKey, sigV4TestSecretKey, "us-east-1", nil)
	c.now = func() time.Time { return sigV4TestTime }

	sign := func(payload string) string {
		req, err := http.NewRequest(http.MethodPost, "https://polly.us-east-1.amazonaws.com/v1/speech", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		c.sign(req, []byte(payload))
		return req.Header.Get("Authorization")
	}

	first, second := sign(`{"Text":"a"}`), sign(`{"Text":"a"}`)
	if first != second {
		t.Fatalf("identical requests signed differently:\n%s\n%s", first, second)
	}
	if other := sign(`{"Text":"b"}`); other == first {
		t.Fatal("different payloads produced the same signature")
	}
	if !strings.HasPrefix(first, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/polly/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=") {
		t.Fatalf("unexpected Authorization shape: %q", first)
	}
}

func TestSigV4SignUsesRequestHost(t *testing.T) {
	c := NewAWSClient(sigV4TestAccessKey, sigV4TestSecretKey, "us-east-1", nil)
	c.now = func() time.Time { return sigV4TestTime }

	sign := func(rawURL string) string {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		c.sign(req, nil)
		return req.Header.Get("Authorization")
	}
	if sign("https://polly.us-east-1.amazonaws.com/v1/voices") == sign("http://127.0.0.1:9999/v1/voices") {
		t.Fatal("signature must depend on the request host")
	}
}

func TestSigV4URIEncode(t *testing.T) {
	cases := map[string]string{
		"abcXYZ019-._~": "abcXYZ019-._~",
		"a b":           "a%20b",
		"a+b":           "a%2Bb",
		"/":             "%2F",
		"en-US":         "en-US",
		"é":             "%C3%A9",
		"*":             "%2A",
	}
	for in, want := range cases {
		if got := sigV4URIEncode(in); got != want {
			t.Fatalf("sigV4URIEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSigV4CanonicalQuerySortsAndEncodes(t *testing.T) {
	q := url.Values{}
	q.Set("NextToken", "a b/c")
	q.Set("LanguageCode", "en-US")
	q.Add("Dup", "2")
	q.Add("Dup", "1")
	got := sigV4CanonicalQuery(q)
	want := "Dup=1&Dup=2&LanguageCode=en-US&NextToken=a%20b%2Fc"
	if got != want {
		t.Fatalf("canonical query = %q, want %q", got, want)
	}
	if sigV4CanonicalQuery(url.Values{}) != "" {
		t.Fatal("empty query must canonicalize to empty string")
	}
}

func TestSigV4CanonicalURI(t *testing.T) {
	if got := sigV4CanonicalURI(""); got != "/" {
		t.Fatalf("empty path = %q, want /", got)
	}
	if got := sigV4CanonicalURI("/v1/speech"); got != "/v1/speech" {
		t.Fatalf("unexpected canonical URI: %q", got)
	}
	if got := sigV4CanonicalURI("/a b/c"); got != "/a%20b/c" {
		t.Fatalf("unexpected canonical URI: %q", got)
	}
}

func TestAWSAPIErrorIsEngineNotSupported(t *testing.T) {
	cases := []struct {
		err  awsAPIError
		want bool
	}{
		{awsAPIError{StatusCode: 400, ErrorType: "EngineNotSupportedException"}, true},
		{awsAPIError{StatusCode: 400, Message: "This engine is not compatible with the voice that you have designated."}, true},
		{awsAPIError{StatusCode: 400, Message: "Voice does not support the neural engine"}, true},
		{awsAPIError{StatusCode: 400, ErrorType: "ValidationException", Message: "bad input"}, false},
		{awsAPIError{StatusCode: 403, ErrorType: "EngineNotSupportedException"}, false},
		{awsAPIError{StatusCode: 500}, false},
	}
	for _, tc := range cases {
		if got := tc.err.isEngineNotSupported(); got != tc.want {
			t.Fatalf("isEngineNotSupported(%#v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func hexHMAC(key []byte, data string) string {
	return fmt.Sprintf("%x", hmacSHA256(key, []byte(data)))
}
