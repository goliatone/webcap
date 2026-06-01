package webcap

import (
	"bufio"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	authSourceCookieJar    = "cookie_jar"
	authSourceStorageState = "storage_state"

	authCookieStatusPresent       = "present"
	authCookieStatusMissing       = "missing"
	authCookieStatusExpired       = "expired"
	authCookieStatusNotApplicable = "not_applicable"
)

func InspectAuthState(req AuthInspectRequest, now time.Time) (AuthInspectResult, error) {
	req = normalizeAuthInspectRequest(req)
	if now.IsZero() {
		now = time.Now()
	}
	if req.CookieJar == "" && req.StorageState == "" {
		return AuthInspectResult{}, newCaptureError(CodeValidation, "auth_inspect", "auth inspect requires cookie jar or storage state", nil)
	}
	if req.CookieJar != "" && req.StorageState != "" {
		return AuthInspectResult{}, newCaptureError(CodeValidation, "auth_inspect", "auth inspect accepts only one auth state source", nil)
	}

	result := AuthInspectResult{
		Command:     "auth inspect",
		TargetURL:   req.TargetURL,
		InspectedAt: now,
	}
	if req.CookieJar != "" {
		cookies, err := loadDiagnosticCookieJar(req.CookieJar, now)
		if err != nil {
			return AuthInspectResult{}, err
		}
		result.Source = AuthStateSource{Type: authSourceCookieJar, Path: req.CookieJar}
		result.Cookies = authCookieDiagnostics(cookies, req.TargetURL, now)
	} else {
		state, err := LoadPlaywrightStorageState(req.StorageState)
		if err != nil {
			return AuthInspectResult{}, err
		}
		result.Source = AuthStateSource{Type: authSourceStorageState, Path: req.StorageState}
		result.HasOriginStorage = state.HasOriginStorage()
		result.Cookies = authCookieDiagnostics(storageStateDiagnostics(state.Cookies, now), req.TargetURL, now)
	}
	result.ExpectedCookies = expectedCookieStatuses(req.ExpectCookies, result.Cookies)
	result.Warnings = authDebugCookieWarnings(req.WarnDebugCookies, req.ExpectCookies, result.Cookies)
	if hasFailedExpectedCookie(result.ExpectedCookies) {
		return result, newCaptureError(CodeValidation, "auth_inspect", "auth state did not satisfy expected cookie checks", nil).
			WithMetadata("source", result.Source).
			WithMetadata("target_url", result.TargetURL).
			WithMetadata("expected_cookies", result.ExpectedCookies)
	}
	return result, nil
}

func normalizeAuthInspectRequest(req AuthInspectRequest) AuthInspectRequest {
	req.CookieJar = strings.TrimSpace(req.CookieJar)
	req.StorageState = strings.TrimSpace(req.StorageState)
	req.TargetURL = strings.TrimSpace(req.TargetURL)
	req.ExpectCookies = normalizeStrings(req.ExpectCookies)
	req.WarnDebugCookies = normalizeStrings(req.WarnDebugCookies)
	if len(req.WarnDebugCookies) == 0 {
		req.WarnDebugCookies = append([]string(nil), DefaultDebugCookieNames...)
	}
	return req
}

type diagnosticCookie struct {
	Cookie          CaptureCookie
	Line            int
	Expired         bool
	Skipped         bool
	StorageState    bool
	CaptureImportOK bool
}

func loadDiagnosticCookieJar(path string, now time.Time) ([]diagnosticCookie, error) {
	file, err := os.Open(strings.TrimSpace(path)) // #nosec G304 -- explicit user-supplied diagnostic path.
	if err != nil {
		return nil, newCaptureError(CodeValidation, "auth_inspect", "cookie jar file is not readable", err).
			WithMetadata("path", strings.TrimSpace(path))
	}
	defer func() {
		_ = file.Close()
	}()

	var cookies []diagnosticCookie
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		cookie, ok, err := parseCookieJarLine(scanner.Text(), line, time.Time{})
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		expired := cookie.Expires > 0 && !now.IsZero() && time.Unix(cookie.Expires, 0).Before(now)
		cookies = append(cookies, diagnosticCookie{
			Cookie:          cookie,
			Line:            line,
			Expired:         expired,
			Skipped:         expired,
			CaptureImportOK: !expired,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, newCaptureError(CodeValidation, "auth_inspect", "cookie jar could not be read", err).
			WithMetadata("path", strings.TrimSpace(path))
	}
	return cookies, nil
}

func storageStateDiagnostics(cookies []CaptureCookie, now time.Time) []diagnosticCookie {
	out := make([]diagnosticCookie, 0, len(cookies))
	for _, cookie := range normalizeCaptureCookies(cookies) {
		expired := cookie.Expires > 0 && !now.IsZero() && time.Unix(cookie.Expires, 0).Before(now)
		out = append(out, diagnosticCookie{
			Cookie:          cookie,
			Expired:         expired,
			Skipped:         expired,
			StorageState:    true,
			CaptureImportOK: !expired,
		})
	}
	return out
}

func authCookieDiagnostics(cookies []diagnosticCookie, target string, now time.Time) []AuthCookieDiagnostic {
	out := make([]AuthCookieDiagnostic, 0, len(cookies))
	for _, item := range cookies {
		diag := AuthCookieDiagnostic{
			Name:            item.Cookie.Name,
			Domain:          item.Cookie.Domain,
			Path:            item.Cookie.Path,
			Secure:          item.Cookie.Secure,
			HTTPOnly:        item.Cookie.HTTPOnly,
			SameSite:        item.Cookie.SameSite,
			Expires:         item.Cookie.Expires,
			Expired:         item.Expired,
			Skipped:         item.Skipped,
			HostOnly:        item.Cookie.HostOnly,
			SourceLine:      item.Line,
			StorageState:    item.StorageState,
			CaptureImportOK: item.CaptureImportOK,
		}
		if !now.IsZero() {
			checkedAt := now
			diag.CheckedAt = &checkedAt
		}
		if strings.TrimSpace(target) != "" {
			applicable, reasons := cookieAppliesToURL(item.Cookie, target, item.Expired)
			diag.Applicable = &applicable
			diag.NotApplicable = reasons
		}
		out = append(out, diag)
	}
	return out
}

func expectedCookieStatuses(expected []string, cookies []AuthCookieDiagnostic) []AuthExpectedCookieStatus {
	expected = normalizeStrings(expected)
	out := make([]AuthExpectedCookieStatus, 0, len(expected))
	for _, name := range expected {
		status := AuthExpectedCookieStatus{Name: name, Status: authCookieStatusMissing, Reasons: []string{"absent"}}
		var expiredMatch *AuthCookieDiagnostic
		var notApplicableMatch *AuthCookieDiagnostic
		for i := range cookies {
			cookie := cookies[i]
			if !strings.EqualFold(cookie.Name, name) {
				continue
			}
			if cookie.Expired {
				expiredMatch = &cookie
				continue
			}
			if cookie.Applicable != nil && !*cookie.Applicable {
				notApplicableMatch = &cookie
				continue
			}
			applicable := true
			if cookie.Applicable != nil {
				applicable = *cookie.Applicable
			}
			status = AuthExpectedCookieStatus{
				Name:        name,
				Status:      authCookieStatusPresent,
				Applicable:  &applicable,
				MatchedName: cookie.Name,
			}
			break
		}
		if status.Status == authCookieStatusMissing && expiredMatch != nil {
			applicable := false
			status = AuthExpectedCookieStatus{
				Name:        name,
				Status:      authCookieStatusExpired,
				Applicable:  &applicable,
				Reasons:     appendReason(expiredMatch.NotApplicable, "expired"),
				MatchedName: expiredMatch.Name,
			}
		}
		if status.Status == authCookieStatusMissing && notApplicableMatch != nil {
			applicable := false
			status = AuthExpectedCookieStatus{
				Name:        name,
				Status:      authCookieStatusNotApplicable,
				Applicable:  &applicable,
				Reasons:     append([]string(nil), notApplicableMatch.NotApplicable...),
				MatchedName: notApplicableMatch.Name,
			}
		}
		out = append(out, status)
	}
	return out
}

func appendReason(reasons []string, reason string) []string {
	out := append([]string(nil), reasons...)
	if slices.Contains(out, reason) {
		return out
	}
	return append(out, reason)
}

func hasFailedExpectedCookie(statuses []AuthExpectedCookieStatus) bool {
	for _, status := range statuses {
		if status.Status != authCookieStatusPresent {
			return true
		}
	}
	return false
}

func authDebugCookieWarnings(debugCookies, expected []string, cookies []AuthCookieDiagnostic) []AuthDiagnosticWarning {
	debugCookies = normalizeStrings(debugCookies)
	if len(debugCookies) == 0 {
		return nil
	}
	expectedStatuses := expectedCookieStatuses(expected, cookies)
	hasExpected := len(expected) == 0
	for _, status := range expectedStatuses {
		if status.Status == authCookieStatusPresent {
			hasExpected = true
			break
		}
	}
	if hasExpected {
		return nil
	}
	var warnings []AuthDiagnosticWarning
	for _, debug := range debugCookies {
		for _, cookie := range cookies {
			if !strings.EqualFold(cookie.Name, debug) || cookie.Expired {
				continue
			}
			warnings = append(warnings, AuthDiagnosticWarning{
				Code:    "debug_cookie_without_auth",
				Message: "debug cookie is present but expected auth cookies are not present",
				Cookie:  cookie.Name,
			})
			break
		}
	}
	return warnings
}

func cookieAppliesToURL(cookie CaptureCookie, target string, expired bool) (bool, []string) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false, []string{"invalid_target_url"}
	}
	var reasons []string
	if expired {
		reasons = append(reasons, "expired")
	}
	host := strings.ToLower(hostWithoutPort(parsed.Host))
	domain := strings.ToLower(strings.TrimSpace(cookie.Domain))
	if !cookieDomainMatches(host, domain, cookie.HostOnly) {
		reasons = append(reasons, "domain_mismatch")
	}
	if !cookiePathMatches(parsed.EscapedPath(), cookie.Path) {
		reasons = append(reasons, "path_mismatch")
	}
	if cookie.Secure && parsed.Scheme != "https" {
		reasons = append(reasons, "secure_cookie_on_http")
	}
	return len(reasons) == 0, reasons
}

func hostWithoutPort(hostport string) string {
	host := strings.TrimSpace(hostport)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return strings.Trim(strings.ToLower(host), "[]")
}

func cookieDomainMatches(host, domain string, hostOnly bool) bool {
	domain = strings.TrimPrefix(strings.TrimSpace(domain), ".")
	if domain == "" {
		return host != ""
	}
	if hostOnly {
		return strings.EqualFold(host, domain)
	}
	if strings.EqualFold(host, domain) {
		return true
	}
	return strings.HasSuffix(host, "."+strings.ToLower(domain))
}

func cookiePathMatches(requestPath, cookiePath string) bool {
	if requestPath == "" {
		requestPath = "/"
	}
	if cookiePath == "" {
		cookiePath = "/"
	}
	if requestPath == cookiePath {
		return true
	}
	if !strings.HasPrefix(requestPath, cookiePath) {
		return false
	}
	if strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return len(requestPath) > len(cookiePath) && requestPath[len(cookiePath)] == '/'
}
