package webcap

import (
	"context"
	"image/color"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOpenAISemanticProviderBuildsVisionRequest(t *testing.T) {
	body, err := buildOpenAISemanticRequest(SemanticProviderRequest{
		Model:           "gpt-test",
		Prompt:          "Compare",
		MaxOutputTokens: 100,
		StructuredJSON:  true,
		Images: []SemanticImagePayload{{
			MIMEType:   "image/png",
			Base64Data: "abc",
		}},
	})
	if err != nil {
		t.Fatalf("buildOpenAISemanticRequest returned error: %v", err)
	}
	text := string(body)
	for _, expected := range []string{`"model":"gpt-test"`, `"input_image"`, "data:image/png;base64,abc", `"json_object"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected OpenAI body to contain %q: %s", expected, text)
		}
	}
}

func TestOpenAISemanticProviderMissingCredentials(t *testing.T) {
	provider := NewOpenAISemanticDiffProvider(OpenAISemanticProviderOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "", nil },
	})
	if _, err := provider.CompareImages(context.Background(), SemanticProviderRequest{}); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestOpenAISemanticProviderUsesHTTPTransport(t *testing.T) {
	var sawAuth bool
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sawAuth = req.Header.Get("Authorization") == "Bearer test-key"
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	provider := NewOpenAISemanticDiffProvider(OpenAISemanticProviderOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         client,
	})
	resp, err := provider.CompareImages(context.Background(), SemanticProviderRequest{Model: "gpt-test", Prompt: "Compare", MaxOutputTokens: 10})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if !sawAuth || resp.Model != "gpt-test" || resp.Usage.TotalTokens != 3 || !strings.Contains(resp.RawText, `"summary"`) {
		t.Fatalf("unexpected provider response/auth: auth=%v resp=%#v", sawAuth, resp)
	}
}

func TestAnthropicSemanticProviderBuildsVisionRequest(t *testing.T) {
	body, err := buildAnthropicSemanticRequest(SemanticProviderRequest{
		Model:           "claude-test",
		Prompt:          "Compare",
		MaxOutputTokens: 100,
		Images: []SemanticImagePayload{{
			MIMEType:   "image/png",
			Base64Data: "abc",
		}},
	})
	if err != nil {
		t.Fatalf("buildAnthropicSemanticRequest returned error: %v", err)
	}
	text := string(body)
	for _, expected := range []string{`"model":"claude-test"`, `"type":"image"`, `"media_type":"image/png"`, `"data":"abc"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected Anthropic body to contain %q: %s", expected, text)
		}
	}
}

func TestAnthropicSemanticProviderMissingCredentials(t *testing.T) {
	provider := NewAnthropicSemanticDiffProvider(AnthropicSemanticProviderOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "", nil },
	})
	if _, err := provider.CompareImages(context.Background(), SemanticProviderRequest{}); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestAnthropicSemanticProviderUsesHTTPTransport(t *testing.T) {
	var sawKey bool
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sawKey = req.Header.Get("x-api-key") == "test-key" && req.Header.Get("anthropic-version") != ""
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"model":"claude-test","content":[{"type":"text","text":"{\"summary\":\"ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}],"usage":{"input_tokens":1,"output_tokens":2}}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	provider := NewAnthropicSemanticDiffProvider(AnthropicSemanticProviderOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         client,
	})
	resp, err := provider.CompareImages(context.Background(), SemanticProviderRequest{Model: "claude-test", Prompt: "Compare", MaxOutputTokens: 10})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if !sawKey || resp.Model != "claude-test" || resp.Usage.TotalTokens != 3 || !strings.Contains(resp.RawText, `"summary"`) {
		t.Fatalf("unexpected provider response/auth: key=%v resp=%#v", sawKey, resp)
	}
}

func TestSemanticDiffOptionsRegistersBuiltInProviders(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 255, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}

	var sawURL string
	var sawProvider string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sawURL = req.URL.String()
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}]}]}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		CredentialResolver: func(_ context.Context, provider string) (string, error) {
			sawProvider = provider
			return "test-key", nil
		},
		HTTPClient:    client,
		DefaultModels: map[string]string{"openai": "gpt-test"},
		OpenAIBaseURL: "https://example.test/openai",
	}})

	result, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:   currentPath,
		ReferencePath: referencePath,
		Provider:      "openai",
		MetadataPath:  filepath.Join(dir, "semantic.json"),
	})
	if err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if result.Provider != "openai" || result.Model != "gpt-test" || result.Summary != "ok" {
		t.Fatalf("unexpected semantic result: %#v", result)
	}
	if sawProvider != "openai" || sawURL != "https://example.test/openai" {
		t.Fatalf("built-in provider did not receive configured resolver/base URL: provider=%q url=%q", sawProvider, sawURL)
	}
}

func TestSemanticDiffOptionsCallerProviderOverridesBuiltIn(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 255, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}

	provider := &fakeSemanticProvider{name: "openai", resp: SemanticProviderResponse{
		Provider: "custom-openai",
		Model:    "custom-model",
		RawText:  `{"summary":"custom provider","verdict":"needs_review","severity":"minor"}`,
	}}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		Providers:     map[string]SemanticDiffProvider{"OPENAI": provider},
		DefaultModels: map[string]string{"openai": "custom-model"},
		CredentialResolver: func(context.Context, string) (string, error) {
			t.Fatal("caller provider should override built-in and avoid built-in credential resolution")
			return "", nil
		},
	}})

	result, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:   currentPath,
		ReferencePath: referencePath,
		Provider:      "openai",
		MetadataPath:  filepath.Join(dir, "semantic.json"),
	})
	if err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if provider.lastReq.Provider != "openai" || result.Provider != "custom-openai" || result.Summary != "custom provider" {
		t.Fatalf("caller provider was not used: req=%#v result=%#v", provider.lastReq, result)
	}
}
