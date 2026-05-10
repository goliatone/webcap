package llms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicProviderBuildsVisionRequest(t *testing.T) {
	body, err := BuildAnthropicRequest(Request{
		Model:           "claude-test",
		Prompt:          "Compare",
		MaxOutputTokens: 100,
		Images: []Image{{
			MIMEType:   "image/png",
			Base64Data: "abc",
		}},
	})
	if err != nil {
		t.Fatalf("BuildAnthropicRequest returned error: %v", err)
	}
	text := string(body)
	for _, expected := range []string{`"model":"claude-test"`, `"type":"image"`, `"media_type":"image/png"`, `"data":"abc"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected Anthropic body to contain %q: %s", expected, text)
		}
	}
}

func TestAnthropicProviderMissingCredentials(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "", nil },
	})
	if _, err := provider.CompareImages(context.Background(), Request{}); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestAnthropicProviderUsesHTTPTransport(t *testing.T) {
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
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         client,
	})
	resp, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare", MaxOutputTokens: 10})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if !sawKey || resp.Model != "claude-test" || resp.Usage.TotalTokens != 3 || !strings.Contains(resp.RawText, `"summary"`) {
		t.Fatalf("unexpected provider response/auth: key=%v resp=%#v", sawKey, resp)
	}
}

func TestAnthropicProviderRejectsMalformedResponse(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(200, `{`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare"}); err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestAnthropicProviderRejectsEmptyTextResponse(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(200, `{"model":"claude-test","content":[{"type":"text","text":"   "}],"usage":{"input_tokens":1,"output_tokens":2}}`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare"}); err == nil || !strings.Contains(err.Error(), "no text") {
		t.Fatalf("expected empty text response error, got %v", err)
	}
}

func TestAnthropicProviderReportsHTTPError(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(429, `{"error":"rate limited"}`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare"}); err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("expected HTTP status error, got %v", err)
	}
}
