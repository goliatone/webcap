package webcap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func ParseCookieJarFile(path string, now time.Time) ([]CaptureCookie, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, newCaptureError(CodeValidation, "parse_cookie_jar", "cookie jar file is not readable", err).
			WithMetadata("path", path)
	}
	defer func() {
		_ = file.Close()
	}()

	var cookies []CaptureCookie
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		cookie, ok, err := parseCookieJarLine(scanner.Text(), line, now)
		if err != nil {
			return nil, err
		}
		if ok {
			cookies = append(cookies, cookie)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, newCaptureError(CodeValidation, "parse_cookie_jar", "cookie jar could not be read", err).
			WithMetadata("path", path)
	}
	return normalizeCaptureCookies(cookies), nil
}

func parseCookieJarLine(raw string, line int, now time.Time) (CaptureCookie, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return CaptureCookie{}, false, nil
	}
	httpOnly := false
	if strings.HasPrefix(value, "#HttpOnly_") {
		httpOnly = true
		value = strings.TrimPrefix(value, "#HttpOnly_")
	} else if strings.HasPrefix(value, "#") {
		return CaptureCookie{}, false, nil
	}
	fields := strings.Split(value, "\t")
	if len(fields) != 7 {
		return CaptureCookie{}, false, cookieJarLineError(line, "cookie jar line must have 7 tab-separated fields")
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	expires, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return CaptureCookie{}, false, cookieJarLineError(line, "cookie jar expiration must be a unix timestamp")
	}
	if expires > 0 && !now.IsZero() && time.Unix(expires, 0).Before(now) {
		return CaptureCookie{}, false, nil
	}
	if fields[5] == "" {
		return CaptureCookie{}, false, cookieJarLineError(line, "cookie jar cookie name is required")
	}
	if fields[2] == "" || !strings.HasPrefix(fields[2], "/") {
		return CaptureCookie{}, false, cookieJarLineError(line, "cookie jar path must start with /")
	}
	return CaptureCookie{
		Name:     fields[5],
		Value:    fields[6],
		Domain:   fields[0],
		Path:     fields[2],
		Secure:   parseCookieJarBool(fields[3]),
		HTTPOnly: httpOnly,
		Expires:  expires,
		HostOnly: !parseCookieJarBool(fields[1]),
	}, true, nil
}

func cookieJarLineError(line int, message string) error {
	return newCaptureError(CodeValidation, "parse_cookie_jar", message, nil).
		WithMetadata("line", line)
}

func parseCookieJarBool(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "TRUE":
		return true
	default:
		return false
	}
}

type playwrightStorageState struct {
	Cookies []CaptureCookie         `json:"cookies"`
	Origins []playwrightStateOrigin `json:"origins"`
}

type playwrightStateOrigin struct {
	Origin         string                `json:"origin"`
	LocalStorage   []playwrightStateItem `json:"localStorage"`
	SessionStorage []playwrightStateItem `json:"sessionStorage"`
}

type playwrightStateItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func LoadPlaywrightStorageState(path string) (playwrightStorageState, error) {
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return playwrightStorageState{}, newCaptureError(CodeValidation, "parse_storage_state", "storage_state file is not readable", err).
			WithMetadata("path", strings.TrimSpace(path))
	}
	var state playwrightStorageState
	if err := json.Unmarshal(payload, &state); err != nil {
		return playwrightStorageState{}, newCaptureError(CodeValidation, "parse_storage_state", "storage_state is not valid Playwright JSON", err).
			WithMetadata("path", strings.TrimSpace(path))
	}
	state.Cookies = normalizeCaptureCookies(state.Cookies)
	return state, nil
}

func (s playwrightStorageState) HasOriginStorage() bool {
	for _, origin := range s.Origins {
		if len(origin.LocalStorage) > 0 || len(origin.SessionStorage) > 0 {
			return true
		}
	}
	return false
}

func resolveCaptureAuthCookies(req CaptureRequest, now time.Time) ([]CaptureCookie, error) {
	var cookies []CaptureCookie
	if req.Auth.CookieJar != "" {
		jarCookies, err := ParseCookieJarFile(req.Auth.CookieJar, now)
		if err != nil {
			return nil, err
		}
		cookies = append(cookies, jarCookies...)
	}
	cookies = append(cookies, req.Auth.Cookies...)
	return normalizeCaptureCookies(cookies), nil
}

func resolveChromiumStorageStateCookies(req CaptureRequest) ([]CaptureCookie, error) {
	if strings.TrimSpace(req.Auth.StorageState) == "" {
		return nil, nil
	}
	state, err := LoadPlaywrightStorageState(req.Auth.StorageState)
	if err != nil {
		return nil, err
	}
	if state.HasOriginStorage() {
		return nil, newCaptureError(CodeUnsupported, "parse_storage_state", "chromium storage_state support is limited to cookies; origin storage is not supported", nil).
			WithMetadata("engine", "chromium").
			WithMetadata("path", req.Auth.StorageState)
	}
	return state.Cookies, nil
}

func cookieURLForTarget(target string, cookie CaptureCookie) string {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.TrimSpace(cookie.Domain), ".")
	if host == "" || strings.EqualFold(host, "localhost") {
		host = parsed.Host
	}
	path := cookie.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("%s://%s%s", parsed.Scheme, host, path)
}
