package llms

import (
	"context"
	"errors"
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

func TestAnthropicProviderRejectsOversizedRequestBodyBeforeHTTP(t *testing.T) {
	called := false
	provider := NewAnthropicProvider(AnthropicOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		})},
	})
	_, err := provider.CompareImages(context.Background(), Request{
		Model:               "claude-test",
		Prompt:              strings.Repeat("x", 200),
		MaxOutputTokens:     10,
		MaxRequestBodyBytes: 20,
	})
	var budgetErr *PayloadBudgetError
	if !errors.As(err, &budgetErr) || budgetErr.LimitName != "max_request_body_bytes" || called {
		t.Fatalf("expected preflight request budget error without HTTP call, called=%v err=%v budget=%#v", called, err, budgetErr)
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
		HTTPClient: staticHTTPClientWithHeaders(403, `{"type":"error","error":{"type":"permission_error","message":"quota exceeded"}}`, http.Header{
			"Anthropic-Request-Id":      []string{"req-anthropic"},
			"Anthropic-RateLimit-Limit": []string{"50"},
		}),
	})
	_, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare"})
	var httpErr *ProviderHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected typed HTTP error, got %T %[1]v", err)
	}
	if httpErr.Provider != ProviderAnthropic || httpErr.StatusCode != 403 || httpErr.RequestID != "req-anthropic" || httpErr.ErrorType != "permission_error" {
		t.Fatalf("unexpected HTTP diagnostics: %#v", httpErr)
	}
	if httpErr.RateLimit["anthropic_ratelimit_limit"] != "50" || strings.Contains(httpErr.Error(), "test-key") {
		t.Fatalf("unexpected safe metadata: err=%v meta=%#v", httpErr, httpErr.Metadata())
	}
}

func TestAnthropicProviderHTTPErrorStatuses(t *testing.T) {
	for _, status := range []int{400, 401, 403, 413, 429, 503} {
		provider := NewAnthropicProvider(AnthropicOptions{
			CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
			HTTPClient:         staticHTTPClient(status, `{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`),
		})
		_, err := provider.CompareImages(context.Background(), Request{Model: "claude-test", Prompt: "Compare"})
		var httpErr *ProviderHTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != status || httpErr.ErrorType != "invalid_request_error" {
			t.Fatalf("status %d: expected typed HTTP diagnostics, got err=%v http=%#v", status, err, httpErr)
		}
	}
}
