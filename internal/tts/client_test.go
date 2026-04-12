package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
		if got := r.URL.Query().Get("key"); got != "k" {
			t.Fatalf("unexpected api key query: %q", got)
		}
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
	c := NewClient("a+b/c=1", nil)
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
	if got := parsed.Query().Get("key"); got != "a+b/c=1" {
		t.Fatalf("unexpected key param: %q", got)
	}
	if got := parsed.Query().Get("languageCode"); got != "en US/TH" {
		t.Fatalf("unexpected languageCode param: %q", got)
	}
}
