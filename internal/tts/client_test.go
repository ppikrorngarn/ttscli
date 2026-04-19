package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTTSClientSynthesizeSuccess(t *testing.T) {
	audio := []byte("hello-audio")
	audioB64 := base64.StdEncoding.EncodeToString(audio)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiPathSynthesize {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertAPIKeyHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"audioContent":"%s"}`, audioB64)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	got, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("unexpected audio payload: %q", string(got))
	}
}

func TestTTSClientSynthesizeRequestPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		var req synthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request payload: %v", err)
		}
		if req.Input.Text != "hello world" {
			t.Fatalf("unexpected input text: %q", req.Input.Text)
		}
		if req.Voice.LanguageCode != "en-GB" {
			t.Fatalf("unexpected language code: %q", req.Voice.LanguageCode)
		}
		if req.Voice.Name != "en-GB-Neural2-B" {
			t.Fatalf("unexpected voice name: %q", req.Voice.Name)
		}
		if req.AudioConfig.AudioEncoding != AudioEncodingMP3 {
			t.Fatalf("unexpected audio encoding: %q", req.AudioConfig.AudioEncoding)
		}
		fmt.Fprint(w, `{"audioContent":"aGVsbG8="}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Synthesize(context.Background(), "hello world", "en-GB", "en-GB-Neural2-B", AudioEncodingMP3)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
}

func TestTTSClientSynthesizeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "status=400") {
		t.Fatalf("expected status error, got: %v", err)
	}
}

func TestTTSClientSynthesizeInvalidBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"audioContent":"$$$"}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "decode audioContent") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestTTSClientSynthesizeInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid-json`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestTTSClientSynthesizeEmptyAudioContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"audioContent":""}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "empty audioContent") {
		t.Fatalf("expected empty audioContent error, got: %v", err)
	}
}

func TestTTSClientListVoicesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiPathVoices {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertAPIKeyHeader(t, r)
		if got := r.URL.Query().Get("languageCode"); got != "en-GB" {
			t.Fatalf("unexpected languageCode query: %q", got)
		}
		fmt.Fprint(w, `{"voices":[{"languageCodes":["en-GB"],"name":"en-GB-Neural2-B","ssmlGender":"MALE"}]}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	voices, err := c.ListVoices(context.Background(), "en-GB")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
	if len(voices) != 1 || voices[0].Name != "en-GB-Neural2-B" {
		t.Fatalf("unexpected voices: %#v", voices)
	}
}

func TestTTSClientListVoicesOmitsLanguageCodeWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		if got := r.URL.Query().Get("languageCode"); got != "" {
			t.Fatalf("expected no languageCode query, got %q", got)
		}
		fmt.Fprint(w, `{"voices":[]}`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.ListVoices(context.Background(), "")
	if err != nil {
		t.Fatalf("ListVoices returned error: %v", err)
	}
}

func TestTTSClientListVoicesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid-json`)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.ListVoices(context.Background(), "en-US")
	if err == nil || !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestTTSClientBuildRequestURLInvalidBaseURL(t *testing.T) {
	c := NewClient("k", nil)
	c.baseURL = "://invalid-base-url"

	_, err := c.buildRequestURL(apiPathVoices, nil)
	if err == nil {
		t.Fatal("expected invalid base URL error")
	}
}

func TestTTSClientBuildRequestURLEncodesQueryParams(t *testing.T) {
	c := NewClient("k", nil)
	c.baseURL = "https://example.com"

	extra := url.Values{}
	extra.Set("languageCode", "en US/TH")

	rawURL, err := c.buildRequestURL(apiPathVoices, extra)
	if err != nil {
		t.Fatalf("buildRequestURL returned error: %v", err)
	}
	if strings.Contains(rawURL, " ") {
		t.Fatalf("URL must not contain raw spaces: %q", rawURL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	if got := parsed.Query().Get("languageCode"); got != "en US/TH" {
		t.Fatalf("unexpected languageCode param: %q", got)
	}
}

func TestTTSClientSynthesizeHTTPCallError(t *testing.T) {
	c := NewClient("k", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assertAPIKeyHeader(t, req)
			return nil, errors.New("network down")
		}),
	})
	c.baseURL = "https://example.com"

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "http call") {
		t.Fatalf("expected http call error, got: %v", err)
	}
}

func TestTTSClientSynthesizeReadResponseError(t *testing.T) {
	c := NewClient("k", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assertAPIKeyHeader(t, req)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(errReader{}),
				Header:     make(http.Header),
			}, nil
		}),
	})
	c.baseURL = "https://example.com"

	_, err := c.Synthesize(context.Background(), "hello", "en-US", "en-US-Neural2-F", AudioEncodingMP3)
	if err == nil || !strings.Contains(err.Error(), "read response") {
		t.Fatalf("expected read response error, got: %v", err)
	}
}

func TestTTSClientListVoicesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIKeyHeader(t, r)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient("k", srv.Client())
	c.baseURL = srv.URL

	_, err := c.ListVoices(context.Background(), "en-US")
	if err == nil || !strings.Contains(err.Error(), "status=401") {
		t.Fatalf("expected status error, got: %v", err)
	}
}

func TestNewProviderGCP(t *testing.T) {
	p, err := NewProvider("gcp", "my-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "gcp" {
		t.Errorf("expected gcp provider name, got %q", p.Name())
	}
}

func TestNewProviderGCPBadCreds(t *testing.T) {
	_, err := NewProvider("gcp", 123)
	if err == nil || !strings.Contains(err.Error(), "string API key") {
		t.Fatalf("expected credentials error, got: %v", err)
	}
}

func TestNewProviderUnsupported(t *testing.T) {
	for _, name := range []string{"aws", "azure", "ibm", "alibaba"} {
		_, err := NewProvider(name, nil)
		if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
			t.Errorf("provider %q: expected not implemented error, got: %v", name, err)
		}
	}
}

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider("openai", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}

func TestPrintVoices(t *testing.T) {
	voices := []Voice{
		{Name: "en-US-Neural2-F", SsmlGender: "FEMALE", LanguageCodes: []string{"en-US"}},
		{Name: "en-GB-Neural2-B", SsmlGender: "MALE", LanguageCodes: []string{"en-GB"}},
	}
	var buf bytes.Buffer
	PrintVoices(&buf, "en-US", voices)
	out := buf.String()
	if !strings.Contains(out, "en-US-Neural2-F") || !strings.Contains(out, "FEMALE") {
		t.Errorf("expected voice details in output, got: %q", out)
	}
	if !strings.Contains(out, "en-US") {
		t.Errorf("expected language code in header, got: %q", out)
	}
}

func assertAPIKeyHeader(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("X-Goog-Api-Key"); got != "k" {
		t.Fatalf("unexpected api key header: %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}
