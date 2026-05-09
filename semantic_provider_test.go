package webcap

import (
	"context"
	"io"
	"net/http"
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
