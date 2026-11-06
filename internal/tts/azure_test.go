package tts

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// azureVoicesFixture is trimmed from the sample response in
// https://learn.microsoft.com/azure/ai-services/speech-service/rest-text-to-speech
// (Get a list of voices > Sample response), with one extra en-GB voice so the
// filter has something to exclude.
const azureVoicesFixture = `[
    {
        "Name": "Microsoft Server Speech Text to Speech Voice (en-US, JennyNeural)",
        "DisplayName": "Jenny",
        "LocalName": "Jenny",
        "ShortName": "en-US-JennyNeural",
        "Gender": "Female",
        "Locale": "en-US",
        "LocaleName": "English (United States)",
        "StyleList": ["assistant", "chat", "customerservice", "newscast"],
        "SampleRateHertz": "48000",
        "VoiceType": "Neural",
        "Status": "GA",
        "WordsPerMinute": "152"
    },
    {
        "Name": "Microsoft Server Speech Text to Speech Voice (en-US, JennyMultilingualNeural)",
        "DisplayName": "Jenny Multilingual",
        "LocalName": "Jenny Multilingual",
        "ShortName": "en-US-JennyMultilingualNeural",
        "Gender": "Female",
        "Locale": "en-US",
        "LocaleName": "English (United States)",
        "SecondaryLocaleList": ["de-DE", "en-AU", "en-CA", "en-GB", "es-ES", "fr-FR", "ja-JP", "zh-CN"],
        "SampleRateHertz": "48000",
        "VoiceType": "Neural",
        "Status": "GA",
        "WordsPerMinute": "190"
    },
    {
        "Name": "Microsoft Server Speech Text to Speech Voice (ga-IE, OrlaNeural)",
        "DisplayName": "Orla",
        "LocalName": "Orla",
        "ShortName": "ga-IE-OrlaNeural",
        "Gender": "Female",
        "Locale": "ga-IE",
        "LocaleName": "Irish (Ireland)",
        "SampleRateHertz": "48000",
        "VoiceType": "Neural",
        "Status": "GA",
        "WordsPerMinute": "139"
    },
    {
        "Name": "Microsoft Server Speech Text to Speech Voice (zh-CN, YunxiNeural)",
        "DisplayName": "Yunxi",
        "LocalName": "云希",
        "ShortName": "zh-CN-YunxiNeural",
        "Gender": "Male",
        "Locale": "zh-CN",
        "LocaleName": "Chinese (Mandarin, Simplified)",
        "StyleList": ["narration-relaxed", "embarrassed", "fearful"],
        "SampleRateHertz": "48000",
        "VoiceType": "Neural",
        "Status": "GA",
        "RolePlayList": ["Narrator", "YoungAdultMale", "Boy"],
        "WordsPerMinute": "293"
    },
    {
        "Name": "Microsoft Server Speech Text to Speech Voice (en-GB, RyanNeural)",
        "DisplayName": "Ryan",
        "LocalName": "Ryan",
        "ShortName": "en-GB-RyanNeural",
        "Gender": "Male",
        "Locale": "en-GB",
        "LocaleName": "English (United Kingdom)",
        "SampleRateHertz": "48000",
        "VoiceType": "Neural",
        "Status": "GA",
        "WordsPerMinute": "150"
    }
]`

// azureSSML mirrors the request body so tests can decode and assert it.
type azureSSML struct {
	XMLName xml.Name `xml:"speak"`
	Version string   `xml:"version,attr"`
	Lang    string   `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Voice   struct {
		Name string `xml:"name,attr"`
		Text string `xml:",chardata"`
	} `xml:"voice"`
}

func newAzureTestClient(t *testing.T, srv *httptest.Server) *AzureClient {
	t.Helper()
	c := NewAzureClient("k", "eastus", srv.Client())
	c.baseURL = srv.URL
	return c
}

func assertAzureAuthHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Ocp-Apim-Subscription-Key"); got != "k" {
		t.Fatalf("unexpected subscription key header: %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != azureUserAgent {
		t.Fatalf("unexpected User-Agent header: %q", got)
	}
}

func TestAzureClientDefaultBaseURL(t *testing.T) {
	c := NewAzureClient("k", "southeastasia", nil)
	if c.baseURL != "https://southeastasia.tts.speech.microsoft.com" {
		t.Fatalf("unexpected base URL: %q", c.baseURL)
	}
	if c.httpClient == nil || c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("expected default http client with %v timeout, got %#v", defaultHTTPTimeout, c.httpClient)
	}
	if c.Name() != "azure" {
		t.Fatalf("unexpected provider name: %q", c.Name())
	}
}

func TestAzureClientSynthesizeSuccess(t *testing.T) {
	audio := []byte("\xff\xfbmp3-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != azurePathSynthesize {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertAzureAuthHeaders(t, r)
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write(audio)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          "hello",
		LanguageCode:  "en-US",
		VoiceName:     "en-US-JennyNeural",
		AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("unexpected audio payload: %q", string(got))
	}
}

func TestAzureClientSynthesizeRequestPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		if got := r.Header.Get("Content-Type"); got != "application/ssml+xml" {
			t.Fatalf("unexpected Content-Type: %q", got)
		}
		if got := r.Header.Get("X-Microsoft-OutputFormat"); got != "audio-24khz-96kbitrate-mono-mp3" {
			t.Fatalf("unexpected X-Microsoft-OutputFormat: %q", got)
		}

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(raw), `xmlns='http://www.w3.org/2001/10/synthesis'`) {
			t.Fatalf("missing SSML namespace in body: %s", raw)
		}
		if !strings.Contains(string(raw), "Tom &amp; Jerry &lt;3 &#34;quotes&#34;") {
			t.Fatalf("text was not XML-escaped: %s", raw)
		}

		var ssml azureSSML
		if err := xml.Unmarshal(raw, &ssml); err != nil {
			t.Fatalf("decode SSML body: %v\n%s", err, raw)
		}
		if ssml.Version != "1.0" {
			t.Fatalf("unexpected speak version: %q", ssml.Version)
		}
		if ssml.Lang != "en-GB" {
			t.Fatalf("unexpected xml:lang: %q", ssml.Lang)
		}
		if ssml.Voice.Name != "en-GB-RyanNeural" {
			t.Fatalf("unexpected voice name: %q", ssml.Voice.Name)
		}
		if ssml.Voice.Text != `Tom & Jerry <3 "quotes"` {
			t.Fatalf("unexpected text: %q", ssml.Voice.Text)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          `Tom & Jerry <3 "quotes"`,
		LanguageCode:  "en-GB",
		VoiceName:     "en-GB-RyanNeural",
		AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestAzureClientSynthesizeUsesDefaultVoiceWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var ssml azureSSML
		if err := xml.Unmarshal(raw, &ssml); err != nil {
			t.Fatalf("decode SSML body: %v", err)
		}
		if ssml.Voice.Name != "en-US-JennyNeural" {
			t.Fatalf("expected default voice, got %q", ssml.Voice.Name)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          "hi",
		LanguageCode:  "en-us",
		AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestAzureClientSynthesizeDerivesLangFromVoice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var ssml azureSSML
		if err := xml.Unmarshal(raw, &ssml); err != nil {
			t.Fatalf("decode SSML body: %v", err)
		}
		if ssml.Lang != "th-TH" {
			t.Fatalf("expected xml:lang derived from voice, got %q", ssml.Lang)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          "สวัสดี",
		VoiceName:     "th-TH-PremwadeeNeural",
		AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestAzureClientSynthesizeRequiresVoiceForOtherLanguages(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          "hallo",
		LanguageCode:  "de-DE",
		AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "--voice") || !strings.Contains(err.Error(), "de-DE") {
		t.Fatalf("expected a --voice hint mentioning the language, got: %v", err)
	}
	if called {
		t.Fatal("no request must be sent when the voice is missing")
	}
}

func TestAzureClientSynthesizeRejectsOtherEncodings(t *testing.T) {
	c := NewAzureClient("k", "eastus", nil)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text:          "hi",
		LanguageCode:  "en-US",
		VoiceName:     "en-US-JennyNeural",
		AudioEncoding: "OGG_OPUS",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported audio encoding") {
		t.Fatalf("expected unsupported encoding error, got: %v", err)
	}
}

func TestAzureClientSynthesizeErrorStatusEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	want := "tts request failed with status 401"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAzureClientSynthesizeErrorStatusPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Unsupported voice en-US-NopeNeural.\n")
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-NopeNeural", AudioEncoding: AudioEncodingMP3,
	})
	want := "tts request failed with status 400: Unsupported voice en-US-NopeNeural."
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAzureClientSynthesizeErrorStatusJSONMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"429","message":"Rate limit is exceeded."}}`)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	want := "tts request failed with status 429: Rate limit is exceeded."
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAzureClientSynthesizeErrorStatusTruncatesLongBody(t *testing.T) {
	long := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, long)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil {
		t.Fatal("expected status error")
	}
	if !strings.HasPrefix(err.Error(), "tts request failed with status 502: ") {
		t.Fatalf("unexpected error prefix: %v", err)
	}
	if len(err.Error()) > 300 || !strings.HasSuffix(err.Error(), "...") {
		t.Fatalf("expected truncated body, got %d chars: %v", len(err.Error()), err)
	}
}

func TestAzureClientSynthesizeNonAudioBodyOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body>Sign in to continue</body></html>")
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "non-audio response") || !strings.Contains(err.Error(), "Sign in to continue") {
		t.Fatalf("expected non-audio error with body preview, got: %v", err)
	}
}

func TestAzureClientSynthesizeEmptyBodyOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "empty audio") {
		t.Fatalf("expected empty audio error, got: %v", err)
	}
}

func TestAzureClientSynthesizeHTTPCallError(t *testing.T) {
	const key = "azure-sentinel-key-12345"
	c := NewAzureClient(key, "eastus", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Ocp-Apim-Subscription-Key"); got != key {
				t.Fatalf("unexpected subscription key header: %q", got)
			}
			if req.URL.Host != "eastus.tts.speech.microsoft.com" {
				t.Fatalf("unexpected host: %q", req.URL.Host)
			}
			return nil, fmt.Errorf("network down")
		}),
	})

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US-JennyNeural", AudioEncoding: AudioEncodingMP3,
	})
	if err == nil || !strings.Contains(err.Error(), "http call") {
		t.Fatalf("expected http call error, got: %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("subscription key leaked into transport error: %v", err)
	}
}

func TestAzureClientListVoicesParsesAndFiltersCaseInsensitively(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != azurePathVoices {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatalf("expected no query string, got %q", r.URL.RawQuery)
		}
		assertAzureAuthHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, azureVoicesFixture)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "EN-us")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}

	want := []Voice{
		{Name: "en-US-JennyMultilingualNeural", LanguageCodes: []string{"en-US", "de-DE", "en-AU", "en-CA", "en-GB", "es-ES", "fr-FR", "ja-JP", "zh-CN"}, SsmlGender: "FEMALE"},
		{Name: "en-US-JennyNeural", LanguageCodes: []string{"en-US"}, SsmlGender: "FEMALE"},
	}
	if !reflect.DeepEqual(voices, want) {
		t.Fatalf("unexpected voices:\n got %#v\nwant %#v", voices, want)
	}
}

func TestAzureClientListVoicesMatchesSecondaryLocales(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, azureVoicesFixture)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "en-gb")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	names := make([]string, 0, len(voices))
	for _, v := range voices {
		names = append(names, v.Name)
	}
	want := []string{"en-GB-RyanNeural", "en-US-JennyMultilingualNeural"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected voices for en-gb: %v, want %v", names, want)
	}
}

func TestAzureClientListVoicesEmptyLangReturnsAllSorted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		fmt.Fprint(w, azureVoicesFixture)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	names := make([]string, 0, len(voices))
	for _, v := range voices {
		names = append(names, v.Name)
	}
	want := []string{
		"en-GB-RyanNeural",
		"en-US-JennyMultilingualNeural",
		"en-US-JennyNeural",
		"ga-IE-OrlaNeural",
		"zh-CN-YunxiNeural",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected voice order: %v, want %v", names, want)
	}
	if voices[4].SsmlGender != "MALE" || !reflect.DeepEqual(voices[4].LanguageCodes, []string{"zh-CN"}) {
		t.Fatalf("unexpected zh-CN voice mapping: %#v", voices[4])
	}
}

func TestAzureClientListVoicesNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, azureVoicesFixture)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	voices, err := c.ListVoices(context.Background(), "xx-XX")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 0 {
		t.Fatalf("expected no voices, got %#v", voices)
	}
}

func TestAzureClientListVoicesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{not-an-array`)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.ListVoices(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestAzureClientListVoicesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAzureAuthHeaders(t, r)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newAzureTestClient(t, srv)
	_, err := c.ListVoices(context.Background(), "en-US")
	want := "tts request failed with status 401"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestAzureClientDefaultVoice(t *testing.T) {
	c := NewAzureClient("k", "eastus", nil)
	cases := map[string]string{
		"en-US": "en-US-JennyNeural",
		"en-us": "en-US-JennyNeural",
		"EN-US": "en-US-JennyNeural",
		"en-GB": "",
		"th-TH": "",
		"":      "",
	}
	for lang, want := range cases {
		if got := c.DefaultVoice(lang); got != want {
			t.Fatalf("DefaultVoice(%q) = %q, want %q", lang, got, want)
		}
	}
	if azureSpec.DefaultVoice != c.DefaultVoice(azureSpec.DefaultLang) {
		t.Fatalf("spec default voice %q disagrees with DefaultVoice(%q) = %q", azureSpec.DefaultVoice, azureSpec.DefaultLang, c.DefaultVoice(azureSpec.DefaultLang))
	}
}

func TestAzureSpecMetadata(t *testing.T) {
	if azureSpec.Name != "azure" || azureSpec.DisplayName != "Azure Speech" {
		t.Fatalf("unexpected spec identity: %q / %q", azureSpec.Name, azureSpec.DisplayName)
	}
	if azureSpec.DefaultLang != "en-US" || azureSpec.DefaultVoice != "en-US-JennyNeural" {
		t.Fatalf("unexpected spec defaults: %q / %q", azureSpec.DefaultLang, azureSpec.DefaultVoice)
	}
	wantKeys := []string{"subscriptionKey", "region"}
	if len(azureSpec.Credentials) != len(wantKeys) {
		t.Fatalf("expected %d credentials, got %d", len(wantKeys), len(azureSpec.Credentials))
	}
	for i, field := range azureSpec.Credentials {
		if field.Key != wantKeys[i] {
			t.Fatalf("credential %d: key %q, want %q", i, field.Key, wantKeys[i])
		}
		if !strings.HasSuffix(field.Prompt, ": ") {
			t.Fatalf("credential %q prompt must end with \": \", got %q", field.Key, field.Prompt)
		}
		if field.Help == "" {
			t.Fatalf("credential %q has no help text", field.Key)
		}
	}
	if !azureSpec.Credentials[0].Secret || azureSpec.Credentials[1].Secret {
		t.Fatal("subscriptionKey must be secret and region must not be")
	}
}

func TestAzureSpecNewBuildsWorkingClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Ocp-Apim-Subscription-Key"); got != "secret-key" {
			t.Fatalf("unexpected subscription key header: %q", got)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	provider, err := azureSpec.New(map[string]string{
		"subscriptionKey": "secret-key",
		"region":          " WestEurope ",
	}, srv.Client())
	if err != nil {
		t.Fatalf("Spec.New returned error: %v", err)
	}
	c, ok := provider.(*AzureClient)
	if !ok {
		t.Fatalf("expected *AzureClient, got %T", provider)
	}
	if c.region != "westeurope" || c.baseURL != "https://westeurope.tts.speech.microsoft.com" {
		t.Fatalf("region not normalised into base URL: region=%q baseURL=%q", c.region, c.baseURL)
	}
	if c.httpClient != srv.Client() {
		t.Fatal("Spec.New must pass the supplied http client through")
	}

	c.baseURL = srv.URL
	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hi", LanguageCode: "en-US", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if string(got) != "mp3" {
		t.Fatalf("unexpected audio: %q", got)
	}
}

func TestAzureSpecNewConsumesEveryCredentialKey(t *testing.T) {
	full := map[string]string{}
	for i, field := range azureSpec.Credentials {
		full[field.Key] = fmt.Sprintf("value%d", i)
	}
	base, err := azureSpec.New(full, nil)
	if err != nil {
		t.Fatalf("Spec.New with all credentials returned error: %v", err)
	}

	for _, field := range azureSpec.Credentials {
		creds := make(map[string]string, len(full))
		for k, v := range full {
			creds[k] = v
		}
		delete(creds, field.Key)

		got, err := azureSpec.New(creds, nil)
		if err != nil {
			continue // rejecting a missing credential counts as consuming it
		}
		if reflect.DeepEqual(got, base) {
			t.Fatalf("credential %q is declared by azureSpec but New ignores it", field.Key)
		}
	}
}

func TestAzureSpecNewRejectsBadRegion(t *testing.T) {
	for _, region := range []string{"", "   ", "east us", "eastus.evil.example/", "east_us"} {
		_, err := azureSpec.New(map[string]string{"subscriptionKey": "k", "region": region}, nil)
		if err == nil {
			t.Fatalf("expected error for region %q", region)
		}
	}
}

func TestAzureBuildSSMLEscapesEverything(t *testing.T) {
	got := string(buildAzureSSML("en-US'", "x'<voice>", "a<b>&c\"d'"))
	want := `<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US&#39;'><voice name='x&#39;&lt;voice&gt;'>a&lt;b&gt;&amp;c&#34;d&#39;</voice></speak>`
	if got != want {
		t.Fatalf("unexpected SSML:\n got %s\nwant %s", got, want)
	}
	var parsed azureSSML
	if err := xml.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("generated SSML must be well-formed: %v", err)
	}
	if parsed.Voice.Text != "a<b>&c\"d'" {
		t.Fatalf("round-tripped text mismatch: %q", parsed.Voice.Text)
	}
}

func TestAzureLocaleFromVoice(t *testing.T) {
	cases := map[string]string{
		"en-US-JennyNeural":      "en-US",
		"zh-CN-YunxiNeural":      "zh-CN",
		"th-TH-PremwadeeNeural":  "th-TH",
		"wuu-CN-XiaotongNeural":  "wuu-CN",
		"JennyNeural":            "en-US",
		"":                       "en-US",
		"en-US-Jenny-Multi-Test": "en-US",
	}
	for voice, want := range cases {
		if got := azureLocaleFromVoice(voice); got != want {
			t.Fatalf("azureLocaleFromVoice(%q) = %q, want %q", voice, got, want)
		}
	}
}

func TestAzureIsAudioContentType(t *testing.T) {
	cases := map[string]bool{
		"":                          true,
		"audio/mpeg":                true,
		"Audio/MPEG; charset=x":     true,
		"application/octet-stream":  true,
		"application/json":          false,
		"text/plain; charset=utf-8": false,
		"text/html":                 false,
	}
	for ct, want := range cases {
		if got := azureIsAudioContentType(ct); got != want {
			t.Fatalf("azureIsAudioContentType(%q) = %v, want %v", ct, got, want)
		}
	}
}
