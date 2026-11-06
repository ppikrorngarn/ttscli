package tts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ibmInstancePath mirrors the /instances/<id> suffix real IBM service URLs carry.
const ibmInstancePath = "/instances/0a1b2c3d-4e5f-6789-abcd-ef0123456789"

// ibmVoicesFixture follows the field set documented for GET /v1/voices.
const ibmVoicesFixture = `{
  "voices": [
    {
      "name": "en-US_MichaelV3Voice",
      "language": "en-US",
      "gender": "male",
      "description": "Michael: American English male voice. Dnn technology.",
      "customizable": true,
      "supported_features": {"custom_pronunciation": true, "voice_transformation": false},
      "url": "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/0a1b2c3d/v1/voices/en-US_MichaelV3Voice"
    },
    {
      "name": "en-GB_KateV3Voice",
      "language": "en-GB",
      "gender": "female",
      "description": "Kate: British English female voice. Dnn technology.",
      "customizable": true,
      "supported_features": {"custom_pronunciation": true, "voice_transformation": false},
      "url": "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/0a1b2c3d/v1/voices/en-GB_KateV3Voice"
    },
    {
      "name": "en-US_AllisonV3Voice",
      "language": "en-US",
      "gender": "female",
      "description": "Allison: American English female voice. Dnn technology.",
      "customizable": true,
      "supported_features": {"custom_pronunciation": true, "voice_transformation": false},
      "url": "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/0a1b2c3d/v1/voices/en-US_AllisonV3Voice"
    },
    {
      "name": "en-GB_CharlotteV3Voice",
      "language": "en-GB",
      "gender": "female",
      "description": "Charlotte: British English female voice. Dnn technology.",
      "customizable": true,
      "supported_features": {"custom_pronunciation": true, "voice_transformation": false},
      "url": "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/0a1b2c3d/v1/voices/en-GB_CharlotteV3Voice"
    }
  ]
}`

func newIBMTestClient(t *testing.T, handler http.HandlerFunc) (*IBMClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := NewIBMClient("k", srv.URL+ibmInstancePath+"/", srv.Client())
	return c, srv
}

func assertIBMBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	// curl -u "apikey:k" sends exactly this header.
	if got := r.Header.Get("Authorization"); got != "Basic YXBpa2V5Oms=" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
}

func TestIBMClientSynthesizeSuccess(t *testing.T) {
	audio := []byte("ID3-fake-mp3-bytes")
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ibmInstancePath+ibmPathSynthesize {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "audio/mp3")
		w.Write(audio)
	})
	defer srv.Close()

	got, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello", LanguageCode: "en-US", VoiceName: "en-US_MichaelV3Voice", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("unexpected audio payload: %q", string(got))
	}
}

func TestIBMClientSynthesizeRequestShape(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != ibmInstancePath+ibmPathSynthesize {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("voice"); got != "en-GB_KateV3Voice" {
			t.Fatalf("unexpected voice query: %q", got)
		}
		if got := r.Header.Get("Accept"); got != "audio/mp3" {
			t.Fatalf("unexpected Accept header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected Content-Type header: %q", got)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "apikey" || pass != "k" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, user, pass)
		}
		assertIBMBasicAuth(t, r)

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(body) != 1 || body["text"] != "hello world" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		w.Header().Set("Content-Type", "audio/mp3")
		w.Write([]byte("mp3"))
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{
		Text: "hello world", LanguageCode: "en-GB", VoiceName: "en-GB_KateV3Voice", AudioEncoding: AudioEncodingMP3,
	})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestIBMClientSynthesizeOmitsVoiceWhenEmpty(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		if _, present := r.URL.Query()["voice"]; present {
			t.Fatalf("expected no voice query, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "audio/mp3")
		w.Write([]byte("mp3"))
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err != nil {
		t.Fatalf("SynthesizeRequest returned error: %v", err)
	}
}

func TestIBMClientSynthesizeErrorStatusJSON(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code":404,"error":"Model 'en-US_NopeVoice' not found","code_description":"Not Found"}`)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", VoiceName: "en-US_NopeVoice", AudioEncoding: AudioEncodingMP3})
	want := "tts request failed with status 404: Model 'en-US_NopeVoice' not found"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestIBMClientSynthesizeErrorStatusUnauthorized(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"code":401,"error":"Unauthorized"}`)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	want := "tts request failed with status 401: Unauthorized"
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestIBMClientSynthesizeErrorStatusPlainBodyNotLeaked(t *testing.T) {
	const secret = "ibm-sentinel-secret-24680"
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, secret)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err == nil || err.Error() != "tts request failed with status 403" {
		t.Fatalf("expected status-only fallback, got: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("response body leaked into error: %v", err)
	}
}

func TestIBMClientSynthesizeNonAudioBodyOn2xx(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `{"unexpected":"json instead of audio"}`)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err == nil {
		t.Fatal("expected non-audio error")
	}
	if !strings.Contains(err.Error(), "non-audio") || !strings.Contains(err.Error(), `json instead of audio`) {
		t.Fatalf("expected non-audio error with body snippet, got: %v", err)
	}
}

func TestIBMClientSynthesizeNonAudioBodySnippetIsBounded(t *testing.T) {
	long := strings.Repeat("x", 1000)
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, long)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err == nil {
		t.Fatal("expected non-audio error")
	}
	if strings.Contains(err.Error(), long) || len(err.Error()) > ibmErrorSnippetLen+100 {
		t.Fatalf("error should quote at most ~%d bytes, got %d: %v", ibmErrorSnippetLen, len(err.Error()), err)
	}
}

func TestIBMClientSynthesizeEmptyBody(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mp3")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err == nil || !strings.Contains(err.Error(), "empty audio") {
		t.Fatalf("expected empty audio error, got: %v", err)
	}
}

func TestIBMClientSynthesizeHTTPCallError(t *testing.T) {
	const apiKey = "ibm-sentinel-api-key-13579"
	c := NewIBMClient(apiKey, "https://example.com"+ibmInstancePath, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			user, pass, ok := req.BasicAuth()
			if !ok || user != "apikey" || pass != apiKey {
				t.Fatalf("unexpected basic auth: ok=%v user=%q", ok, user)
			}
			return nil, errors.New("network down")
		}),
	})

	_, err := c.SynthesizeRequest(context.Background(), SynthRequest{Text: "hi", AudioEncoding: AudioEncodingMP3})
	if err == nil || !strings.Contains(err.Error(), "http call") {
		t.Fatalf("expected http call error, got: %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("api key leaked into transport error: %v", err)
	}
}

func TestIBMClientListVoicesFiltersByLanguageCaseInsensitively(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != ibmInstancePath+ibmPathVoices {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatalf("expected no query parameters, got %q", r.URL.RawQuery)
		}
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, ibmVoicesFixture)
	})
	defer srv.Close()

	voices, err := c.ListVoices(context.Background(), "EN-gb")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 2 {
		t.Fatalf("expected 2 en-GB voices, got %#v", voices)
	}
	if voices[0].Name != "en-GB_CharlotteV3Voice" || voices[1].Name != "en-GB_KateV3Voice" {
		t.Fatalf("expected voices sorted by name, got %#v", voices)
	}
	for _, v := range voices {
		if len(v.LanguageCodes) != 1 || v.LanguageCodes[0] != "en-GB" {
			t.Fatalf("unexpected language codes: %#v", v)
		}
		if v.SsmlGender != "FEMALE" {
			t.Fatalf("expected upper-case gender, got %#v", v)
		}
	}
}

func TestIBMClientListVoicesEmptyLangReturnsAllSorted(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, ibmVoicesFixture)
	})
	defer srv.Close()

	voices, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	wantNames := []string{"en-GB_CharlotteV3Voice", "en-GB_KateV3Voice", "en-US_AllisonV3Voice", "en-US_MichaelV3Voice"}
	if len(voices) != len(wantNames) {
		t.Fatalf("expected %d voices, got %#v", len(wantNames), voices)
	}
	for i, want := range wantNames {
		if voices[i].Name != want {
			t.Fatalf("voice %d: expected %q, got %q", i, want, voices[i].Name)
		}
	}
	if voices[3].SsmlGender != "MALE" {
		t.Fatalf("expected MALE for Michael, got %#v", voices[3])
	}
}

func TestIBMClientListVoicesNoMatchReturnsEmptyNotNil(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ibmVoicesFixture)
	})
	defer srv.Close()

	voices, err := c.ListVoices(context.Background(), "th-TH")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if voices == nil || len(voices) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", voices)
	}
}

func TestIBMClientListVoicesErrorStatus(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertIBMBasicAuth(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"code":401,"error":"Unauthorized: Access is denied due to invalid credentials.","code_description":"Unauthorized"}`)
	})
	defer srv.Close()

	_, err := c.ListVoices(context.Background(), "en-US")
	want := "tts request failed with status 401: Unauthorized: Access is denied due to invalid credentials."
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got: %v", want, err)
	}
}

func TestIBMClientListVoicesInvalidJSON(t *testing.T) {
	c, srv := newIBMTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid-json`)
	})
	defer srv.Close()

	_, err := c.ListVoices(context.Background(), "en-US")
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestIBMClientDefaultVoice(t *testing.T) {
	c := NewIBMClient("k", "https://example.com"+ibmInstancePath, nil)
	if got := c.DefaultVoice("en-US"); got != "en-US_MichaelV3Voice" {
		t.Fatalf("unexpected default voice for en-US: %q", got)
	}
	if got := c.DefaultVoice("en-us"); got != "en-US_MichaelV3Voice" {
		t.Fatalf("expected case-insensitive match, got %q", got)
	}
	if got := c.DefaultVoice("de-DE"); got != "" {
		t.Fatalf("expected empty default voice for de-DE, got %q", got)
	}
	if c.Name() != "ibm" {
		t.Fatalf("unexpected provider name: %q", c.Name())
	}
}

func TestIBMClientNilHTTPClientUsesDefault(t *testing.T) {
	c := NewIBMClient("k", "https://example.com"+ibmInstancePath+"/", nil)
	if c.httpClient == nil || c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("expected default http client with %v timeout, got %#v", defaultHTTPTimeout, c.httpClient)
	}
	if strings.HasSuffix(c.baseURL, "/") {
		t.Fatalf("expected trailing slash trimmed, got %q", c.baseURL)
	}
}

func TestIBMClientBuildRequestURLKeepsInstancePath(t *testing.T) {
	c := NewIBMClient("k", "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/abc/", nil)
	got, err := c.buildRequestURL(ibmPathSynthesize, map[string][]string{"voice": {"en-US_MichaelV3Voice"}})
	if err != nil {
		t.Fatalf("buildRequestURL returned error: %v", err)
	}
	want := "https://api.us-south.text-to-speech.watson.cloud.ibm.com/instances/abc/v1/synthesize?voice=en-US_MichaelV3Voice"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestIBMClientBuildRequestURLInvalidBaseURL(t *testing.T) {
	c := NewIBMClient("k", "://invalid-base-url", nil)
	if _, err := c.buildRequestURL(ibmPathVoices, nil); err == nil {
		t.Fatal("expected invalid base URL error")
	}
	c = NewIBMClient("k", "no-scheme.example.com/instances/abc", nil)
	if _, err := c.buildRequestURL(ibmPathVoices, nil); err == nil {
		t.Fatal("expected missing scheme error")
	}
}

func TestIBMSpecShape(t *testing.T) {
	if ibmSpec.Name != "ibm" || ibmSpec.DisplayName != "IBM Watson Text to Speech" {
		t.Fatalf("unexpected spec identity: %#v", ibmSpec)
	}
	if ibmSpec.DefaultLang != "en-US" || ibmSpec.DefaultVoice != "en-US_MichaelV3Voice" {
		t.Fatalf("unexpected spec defaults: %q %q", ibmSpec.DefaultLang, ibmSpec.DefaultVoice)
	}
	if len(ibmSpec.Credentials) != 2 || ibmSpec.Credentials[0].Key != "apiKey" || ibmSpec.Credentials[1].Key != "url" {
		t.Fatalf("unexpected credential fields: %#v", ibmSpec.Credentials)
	}
	if !ibmSpec.Credentials[0].Secret || ibmSpec.Credentials[1].Secret {
		t.Fatalf("apiKey must be secret and url must not: %#v", ibmSpec.Credentials)
	}
	for _, field := range ibmSpec.Credentials {
		if !strings.HasSuffix(field.Prompt, ": ") {
			t.Fatalf("prompt for %q must end with \": \": %q", field.Key, field.Prompt)
		}
		if field.Help == "" {
			t.Fatalf("help for %q must not be empty", field.Key)
		}
	}
}

func TestIBMSpecNewBuildsWorkingClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ibmInstancePath+ibmPathVoices {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "apikey" || pass != "spec-key" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, user, pass)
		}
		fmt.Fprint(w, ibmVoicesFixture)
	}))
	defer srv.Close()

	creds := map[string]string{
		"apiKey": "spec-key",
		"url":    srv.URL + ibmInstancePath + "/",
	}
	provider, err := ibmSpec.New(creds, srv.Client())
	if err != nil {
		t.Fatalf("Spec.New returned error: %v", err)
	}
	client, ok := provider.(*IBMClient)
	if !ok {
		t.Fatalf("expected *IBMClient, got %T", provider)
	}
	if client.baseURL != srv.URL+ibmInstancePath {
		t.Fatalf("expected trailing slash trimmed, got %q", client.baseURL)
	}
	if provider.Name() != "ibm" {
		t.Fatalf("unexpected name: %q", provider.Name())
	}
	voices, err := provider.ListVoices(context.Background(), "en-US")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 2 {
		t.Fatalf("expected 2 en-US voices, got %#v", voices)
	}
}

func TestIBMSpecNewConsumesEveryCredentialKey(t *testing.T) {
	full := map[string]string{
		"apiKey": "spec-key",
		"url":    "https://example.com" + ibmInstancePath,
	}
	if len(full) != len(ibmSpec.Credentials) {
		t.Fatalf("test creds map must mirror ibmSpec.Credentials (%d fields)", len(ibmSpec.Credentials))
	}
	for _, field := range ibmSpec.Credentials {
		if _, declared := full[field.Key]; !declared {
			t.Fatalf("spec declares credential %q that this test does not know about", field.Key)
		}
		partial := make(map[string]string, len(full))
		for k, v := range full {
			partial[k] = v
		}
		delete(partial, field.Key)
		if _, err := ibmSpec.New(partial, nil); err == nil {
			t.Fatalf("Spec.New must fail when %q is missing, meaning New never read it", field.Key)
		}
	}
	if _, err := ibmSpec.New(full, nil); err != nil {
		t.Fatalf("Spec.New with every credential returned error: %v", err)
	}
}

func TestIBMSpecNewRejectsInvalidURL(t *testing.T) {
	for _, bad := range []string{"not a url", "api.us-south.text-to-speech.watson.cloud.ibm.com/instances/abc", "ftp://example.com"} {
		_, err := ibmSpec.New(map[string]string{"apiKey": "k", "url": bad}, nil)
		if err == nil {
			t.Fatalf("expected error for url %q", bad)
		}
	}
}

func TestIBMIsAudioContentType(t *testing.T) {
	cases := map[string]bool{
		"":                                true,
		"audio/mp3":                       true,
		"audio/mpeg":                      true,
		"audio/ogg;codecs=opus":           true,
		"application/octet-stream":        true,
		"application/json":                false,
		"application/json; charset=utf-8": false,
		"text/html":                       false,
	}
	for ct, want := range cases {
		if got := isIBMAudioContentType(ct); got != want {
			t.Fatalf("isIBMAudioContentType(%q) = %v, want %v", ct, got, want)
		}
	}
}
