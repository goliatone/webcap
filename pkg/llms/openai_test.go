package llms

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

func TestOpenAIProviderBuildsVisionRequest(t *testing.T) {
	body, err := BuildOpenAIRequest(Request{
		Model:           "gpt-test",
		Prompt:          "Compare",
		MaxOutputTokens: 100,
		StructuredJSON:  true,
		Images: []Image{{
			MIMEType:   "image/png",
			Base64Data: "abc",
		}},
	})
	if err != nil {
		t.Fatalf("BuildOpenAIRequest returned error: %v", err)
	}
	text := string(body)
	for _, expected := range []string{`"model":"gpt-test"`, `"input_image"`, "data:image/png;base64,abc", `"json_object"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected OpenAI body to contain %q: %s", expected, text)
		}
	}
}

func TestOpenAIProviderMissingCredentials(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "", nil },
	})
	if _, err := provider.CompareImages(context.Background(), Request{}); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestOpenAIProviderUsesHTTPTransport(t *testing.T) {
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
	provider := NewOpenAIProvider(OpenAIOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         client,
	})
	resp, err := provider.CompareImages(context.Background(), Request{Model: "gpt-test", Prompt: "Compare", MaxOutputTokens: 10})
	if err != nil {
		t.Fatalf("CompareImages returned error: %v", err)
	}
	if !sawAuth || resp.Model != "gpt-test" || resp.Usage.TotalTokens != 3 || !strings.Contains(resp.RawText, `"summary"`) {
		t.Fatalf("unexpected provider response/auth: auth=%v resp=%#v", sawAuth, resp)
	}
}

func TestOpenAIProviderRejectsMalformedResponse(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(200, `{`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "gpt-test", Prompt: "Compare"}); err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestOpenAIProviderRejectsEmptyTextResponse(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(200, `{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"   "}]}]}`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "gpt-test", Prompt: "Compare"}); err == nil || !strings.Contains(err.Error(), "no text") {
		t.Fatalf("expected empty text response error, got %v", err)
	}
}

func TestOpenAIProviderReportsHTTPError(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "test-key", nil },
		HTTPClient:         staticHTTPClient(500, `{"error":"provider unavailable"}`),
	})
	if _, err := provider.CompareImages(context.Background(), Request{Model: "gpt-test", Prompt: "Compare"}); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected HTTP status error, got %v", err)
	}
}

func staticHTTPClient(status int, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
}
