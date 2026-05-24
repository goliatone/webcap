package llms

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const providerHTTPBodyLimit = 2048

type ProviderHTTPError struct {
	Provider      string
	StatusCode    int
	Status        string
	StatusClass   string
	ContentType   string
	RetryAfter    string
	RequestID     string
	BodyExcerpt   string
	BodyTruncated bool
	ErrorType     string
	ErrorCode     string
	ErrorMessage  string
	RateLimit     map[string]string
}

func NewProviderHTTPError(provider string, resp *http.Response, body []byte) *ProviderHTTPError {
	statusCode := 0
	status := ""
	headers := http.Header{}
	if resp != nil {
		statusCode = resp.StatusCode
		status = resp.Status
		headers = resp.Header
	}
	err := &ProviderHTTPError{
		Provider:      NormalizeProviderName(provider),
		StatusCode:    statusCode,
		Status:        strings.TrimSpace(status),
		StatusClass:   httpStatusClass(statusCode),
		ContentType:   strings.TrimSpace(headers.Get("Content-Type")),
		RetryAfter:    firstHeader(headers, "Retry-After"),
		RequestID:     firstHeader(headers, "x-request-id", "request-id", "openai-request-id", "anthropic-request-id"),
		RateLimit:     safeRateLimitHeaders(headers),
		BodyExcerpt:   limitString(strings.TrimSpace(string(body)), providerHTTPBodyLimit),
		BodyTruncated: len(strings.TrimSpace(string(body))) > providerHTTPBodyLimit,
	}
	err.parseBody(body)
	return err
}

func (e *ProviderHTTPError) Error() string {
	if e == nil {
		return ""
	}
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "provider"
	}
	message := fmt.Sprintf("%s returned HTTP %d", provider, e.StatusCode)
	if e.ErrorCode != "" {
		message += " (" + e.ErrorCode + ")"
	} else if e.ErrorType != "" {
		message += " (" + e.ErrorType + ")"
	}
	if e.RetryAfter != "" {
		message += "; retry_after=" + e.RetryAfter
	}
	return message
}

func (e *ProviderHTTPError) Metadata() map[string]string {
	if e == nil {
		return map[string]string{}
	}
	out := map[string]string{
		"provider":       e.Provider,
		"status_code":    strconv.Itoa(e.StatusCode),
		"status_class":   e.StatusClass,
		"body_excerpt":   e.BodyExcerpt,
		"body_truncated": strconv.FormatBool(e.BodyTruncated),
	}
	if e.RetryAfter != "" {
		out["retry_after"] = e.RetryAfter
	}
	if e.RequestID != "" {
		out["request_id"] = e.RequestID
	}
	if e.ErrorType != "" {
		out["error_type"] = e.ErrorType
	}
	if e.ErrorCode != "" {
		out["error_code"] = e.ErrorCode
	}
	if e.ErrorMessage != "" {
		out["error_message"] = e.ErrorMessage
	}
	maps.Copy(out, e.RateLimit)
	return out
}

func (e *ProviderHTTPError) parseBody(body []byte) {
	var parsed map[string]any
	if len(body) == 0 || json.Unmarshal(body, &parsed) != nil {
		return
	}
	if raw, ok := parsed["error"].(map[string]any); ok {
		e.ErrorType = firstJSONText(raw, "type", "error_type")
		e.ErrorCode = firstJSONText(raw, "code", "error_code")
		e.ErrorMessage = firstJSONText(raw, "message")
		return
	}
	e.ErrorType = firstJSONText(parsed, "type", "error_type")
	e.ErrorCode = firstJSONText(parsed, "code", "error_code")
	e.ErrorMessage = firstJSONText(parsed, "message", "error")
}

func firstJSONText(values map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func httpStatusClass(statusCode int) string {
	if statusCode <= 0 {
		return ""
	}
	return strconv.Itoa(statusCode/100) + "xx"
}

func firstHeader(headers http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(headers.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func safeRateLimitHeaders(headers http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if !strings.HasPrefix(normalized, "x-ratelimit-") && !strings.HasPrefix(normalized, "anthropic-ratelimit-") {
			continue
		}
		if len(values) == 0 {
			continue
		}
		value := strings.TrimSpace(values[0])
		if value == "" {
			continue
		}
		out[strings.ReplaceAll(normalized, "-", "_")] = value
	}
	if len(out) == 0 {
		return nil
	}
	keys := make([]string, 0, len(out))
	for key := range out {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sorted := make(map[string]string, len(out))
	for _, key := range keys {
		sorted[key] = out[key]
	}
	return sorted
}
