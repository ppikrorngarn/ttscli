package tts

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
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
