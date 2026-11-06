package tts

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type fakeProvider struct{ creds map[string]string }

func (f fakeProvider) Name() string { return "fake" }
func (f fakeProvider) SynthesizeRequest(context.Context, SynthRequest) ([]byte, error) {
	return []byte("audio"), nil
}
func (f fakeProvider) ListVoices(context.Context, string) ([]Voice, error) { return nil, nil }
func (f fakeProvider) DefaultVoice(string) string                          { return "fake-voice" }

var fakeSpec = Spec{
	Name:        "fake",
	DisplayName: "Fake Speech",
	Credentials: []CredentialField{
		{Key: "token", Prompt: "Token: ", Help: "Get a token from the fake console", Secret: true},
		{Key: "region", Prompt: "Region: ", Help: "Pick a region"},
	},
	DefaultLang:  "xx-XX",
	DefaultVoice: "fake-voice",
	New: func(creds map[string]string, _ *http.Client) (Provider, error) {
		return fakeProvider{creds: creds}, nil
	},
}

func TestLookupKnownAndUnknown(t *testing.T) {
	spec, ok := Lookup("gcp")
	if !ok || spec.Name != "gcp" || spec.DisplayName == "" {
		t.Fatalf("expected gcp spec, got ok=%v spec=%+v", ok, spec)
	}
	if len(spec.Credentials) != 1 || spec.Credentials[0].Key != "apiKey" || !spec.Credentials[0].Secret {
		t.Fatalf("unexpected gcp credentials: %+v", spec.Credentials)
	}
	if _, ok := Lookup("openai"); ok {
		t.Fatal("expected openai to be unknown")
	}
}

func TestNamesSorted(t *testing.T) {
	defer registerForTest(fakeSpec)()
	defer registerForTest(Spec{Name: "aaa"})()
	got := Names()
	want := []string{"aaa", "fake", "gcp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}

func TestPlannedIsCopy(t *testing.T) {
	first := Planned()
	first[0] = "mutated"
	if Planned()[0] == "mutated" {
		t.Fatal("Planned() must return a copy")
	}
	for _, name := range []string{"aws", "azure", "ibm", "alibaba"} {
		if !IsPlanned(name) {
			t.Errorf("expected %q to be planned", name)
		}
	}
	if IsPlanned("gcp") {
		t.Error("gcp must not be planned")
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("gcp"); err != nil {
		t.Fatalf("gcp should validate, got %v", err)
	}
	err := ValidateName("azure")
	if err == nil || !strings.Contains(err.Error(), "not yet implemented") || !strings.Contains(err.Error(), "Available today: gcp") {
		t.Fatalf("unexpected planned error: %v", err)
	}
	err = ValidateName("openai")
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") || !strings.Contains(err.Error(), "Supported providers: gcp") {
		t.Fatalf("unexpected unknown error: %v", err)
	}
}

func TestNewProviderGCP(t *testing.T) {
	p, err := NewProvider("gcp", map[string]interface{}{"apiKey": " my-api-key "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "gcp" {
		t.Errorf("expected gcp provider name, got %q", p.Name())
	}
	gcp, ok := p.(*GCPClient)
	if !ok {
		t.Fatalf("expected *GCPClient, got %T", p)
	}
	if gcp.apiKey != "my-api-key" {
		t.Errorf("expected trimmed api key, got %q", gcp.apiKey)
	}
}

func TestNewProviderMissingCredential(t *testing.T) {
	for name, creds := range map[string]map[string]interface{}{
		"nil map":   nil,
		"empty":     {},
		"blank":     {"apiKey": "   "},
		"wrong key": {"api_key": "x"},
	} {
		_, err := NewProvider("gcp", creds)
		if err == nil || !strings.Contains(err.Error(), `missing credential "apiKey"`) || !strings.Contains(err.Error(), "Google Cloud Console") {
			t.Errorf("%s: expected missing credential error with help, got: %v", name, err)
		}
	}
}

func TestNewProviderNonStringCredential(t *testing.T) {
	_, err := NewProvider("gcp", map[string]interface{}{"apiKey": 123})
	if err == nil || !strings.Contains(err.Error(), `credential "apiKey" must be a string`) {
		t.Fatalf("expected type error, got: %v", err)
	}
}

func TestNewProviderPlannedAndUnknown(t *testing.T) {
	for _, name := range []string{"aws", "azure", "ibm", "alibaba"} {
		_, err := NewProvider(name, nil)
		if err == nil || !strings.Contains(err.Error(), "not yet implemented") {
			t.Errorf("provider %q: expected not implemented error, got: %v", name, err)
		}
	}
	_, err := NewProvider("openai", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}
}

func TestNewProviderSecondProviderThroughRegistry(t *testing.T) {
	defer registerForTest(fakeSpec)()

	p, err := NewProvider("fake", map[string]interface{}{"token": "t", "region": "r", "extra": "ignored"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fp, ok := p.(fakeProvider)
	if !ok {
		t.Fatalf("expected fakeProvider, got %T", p)
	}
	want := map[string]string{"token": "t", "region": "r"}
	if !reflect.DeepEqual(fp.creds, want) {
		t.Fatalf("creds = %v, want %v (undeclared keys must be dropped)", fp.creds, want)
	}

	_, err = NewProvider("fake", map[string]interface{}{"token": "t"})
	if err == nil || !strings.Contains(err.Error(), `missing credential "region"`) || !strings.Contains(err.Error(), "Pick a region") {
		t.Fatalf("expected region error with help, got: %v", err)
	}
}

func TestNewProviderConstructorError(t *testing.T) {
	boom := errors.New("boom")
	defer registerForTest(Spec{
		Name: "broken",
		New:  func(map[string]string, *http.Client) (Provider, error) { return nil, boom },
	})()
	_, err := NewProvider("broken", nil)
	if !errors.Is(err, boom) {
		t.Fatalf("expected constructor error to pass through, got: %v", err)
	}
}

func TestResolveCredentialsNoFieldsDeclared(t *testing.T) {
	got, err := ResolveCredentials(Spec{Name: "none"}, map[string]interface{}{"anything": "x"})
	if err != nil || len(got) != 0 {
		t.Fatalf("expected empty map and nil error, got %v, %v", got, err)
	}
}
