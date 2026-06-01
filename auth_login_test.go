package webcap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoginAuthStateRunsCustomScriptAndValidatesCookieJar(t *testing.T) {
	requireBash(t)
	now := time.Unix(1700000000, 0)
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "cookies.txt")
	scriptPath := filepath.Join(dir, "custom-login.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
cat > "$WEBCAP_COOKIE_JAR" <<COOKIEJAR
# Netscape HTTP Cookie File
localhost	FALSE	/admin	FALSE	1893456000	admin_session	custom-cookie-secret
COOKIEJAR
echo "password=$WEBCAP_PASSWORD"
echo "Set-Cookie: admin_session=custom-cookie-secret"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "super-secret")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    scriptPath,
		BaseURL:       "http://localhost:9090",
		TargetURL:     "http://localhost:9090/admin/dashboard",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
		Timeout:       "5s",
	}, now)
	if err != nil {
		t.Fatalf("LoginAuthState returned error: %v", err)
	}
	if result.Script.Mode != "custom" || result.Script.Path != scriptPath {
		t.Fatalf("unexpected script result: %#v", result.Script)
	}
	if len(result.Inspection.ExpectedCookies) != 1 || result.Inspection.ExpectedCookies[0].Status != authCookieStatusPresent {
		t.Fatalf("expected validated cookie: %#v", result.Inspection.ExpectedCookies)
	}
	assertNoSecretLeak(t, result, "super-secret", "custom-cookie-secret")
}

func TestLoginAuthStateEmbeddedScriptAgainstCSRFServer(t *testing.T) {
	requireBash(t)
	requireCurl(t)
	now := time.Unix(1700000000, 0)
	server := newAuthLoginTestServer(t, "header")
	jarPath := filepath.Join(t.TempDir(), "cookies.txt")
	t.Setenv("ADMIN_PASSWORD", "correct-password")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       server.URL,
		TargetURL:     server.URL + "/admin/dashboard",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
		Timeout:       "10s",
	}, now)
	if err != nil {
		t.Fatalf("LoginAuthState returned error: %v", err)
	}
	if result.Script.Mode != "embedded_go_admin" || result.Script.Path != "" {
		t.Fatalf("unexpected embedded script result: %#v", result.Script)
	}
	if len(result.Inspection.Cookies) == 0 {
		t.Fatalf("expected cookie diagnostics: %#v", result.Inspection)
	}
	assertNoSecretLeak(t, result, "correct-password")
}

func TestAuthHelperWorkflowLoginInspectAndCaptureProtectedRoute(t *testing.T) {
	requireBash(t)
	requireCurl(t)
	now := time.Unix(1700000000, 0)
	server := newAuthLoginTestServer(t, "header")
	jarPath := filepath.Join(t.TempDir(), "cookies.txt")
	targetURL := server.URL + "/admin/dashboard"
	t.Setenv("ADMIN_PASSWORD", "correct-password")

	loginResult, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       server.URL,
		TargetURL:     targetURL,
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
		Timeout:       "10s",
	}, now)
	if err != nil {
		t.Fatalf("LoginAuthState returned error: %v", err)
	}
	if len(loginResult.Inspection.ExpectedCookies) != 1 || loginResult.Inspection.ExpectedCookies[0].Status != authCookieStatusPresent {
		t.Fatalf("login did not validate expected cookie: %#v", loginResult.Inspection.ExpectedCookies)
	}

	inspectResult, err := InspectAuthState(AuthInspectRequest{
		CookieJar:     jarPath,
		TargetURL:     targetURL,
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err != nil {
		t.Fatalf("InspectAuthState returned error: %v", err)
	}
	if len(inspectResult.ExpectedCookies) != 1 || inspectResult.ExpectedCookies[0].Status != authCookieStatusPresent {
		t.Fatalf("inspect did not validate expected cookie: %#v", inspectResult.ExpectedCookies)
	}

	engine := NewChromiumEngine(ChromiumOptions{Headless: true})
	captureResult, err := engine.Capture(context.Background(), CaptureRequest{
		URL: targetURL,
		Auth: CaptureAuth{
			CookieJar: jarPath,
		},
		Guards: CaptureGuards{
			ExpectURL:      "/admin/dashboard",
			FailOnURL:      []string{"/admin/login"},
			FailOnSelector: []string{"form.login"},
		},
		Timeout: "5s",
	})
	if err != nil {
		skipIfChromiumUnavailable(t, err)
		t.Fatalf("Capture returned error: %v", err)
	}
	if !strings.Contains(captureResult.Artifact.URL, "/admin/dashboard") || len(captureResult.Artifact.Bytes) == 0 {
		t.Fatalf("expected protected capture, got %#v", captureResult.Artifact)
	}
	if len(captureResult.Guards) != 3 {
		t.Fatalf("expected three guard outcomes, got %#v", captureResult.Guards)
	}
	assertNoSecretLeak(t, loginResult, "correct-password", "server-cookie-secret")
	assertNoSecretLeak(t, inspectResult, "server-cookie-secret")
}

func TestLoginAuthStateCustomScriptSupportsAdminUserCookieName(t *testing.T) {
	requireBash(t)
	now := time.Unix(1700000000, 0)
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "cookies.txt")
	scriptPath := filepath.Join(dir, "custom-login.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
cat > "$WEBCAP_COOKIE_JAR" <<COOKIEJAR
# Netscape HTTP Cookie File
localhost	FALSE	/admin	FALSE	1893456000	admin_user	admin-user-cookie-secret
COOKIEJAR
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "super-secret")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    scriptPath,
		BaseURL:       "http://localhost:9090",
		TargetURL:     "http://localhost:9090/admin/dashboard",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_user"},
		LoginPath:     DefaultAuthLoginPath,
	}, now)
	if err != nil {
		t.Fatalf("LoginAuthState returned error: %v", err)
	}
	if len(result.Inspection.ExpectedCookies) != 1 || result.Inspection.ExpectedCookies[0].Status != authCookieStatusPresent {
		t.Fatalf("expected admin_user cookie validation: %#v", result.Inspection.ExpectedCookies)
	}
	assertNoSecretLeak(t, result, "super-secret", "admin-user-cookie-secret")
}

func TestLoginAuthStateEmbeddedScriptExtractsHiddenCSRFInput(t *testing.T) {
	requireBash(t)
	requireCurl(t)
	server := newAuthLoginTestServer(t, "hidden")
	jarPath := filepath.Join(t.TempDir(), "cookies.txt")
	t.Setenv("ADMIN_PASSWORD", "correct-password")

	if _, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       server.URL,
		TargetURL:     server.URL + "/admin/dashboard",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
		Timeout:       "10s",
	}, time.Unix(1700000000, 0)); err != nil {
		t.Fatalf("LoginAuthState hidden CSRF returned error: %v", err)
	}
}

func TestLoginAuthStateFailuresAreStructuredAndRedacted(t *testing.T) {
	requireBash(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fail-login.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
echo "Authorization: Bearer raw-token" >&2
echo "password=$WEBCAP_PASSWORD" >&2
exit 9
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "super-secret")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    scriptPath,
		BaseURL:       "http://localhost:9090",
		CookieJar:     filepath.Join(dir, "cookies.txt"),
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
	}, time.Unix(1700000000, 0))
	if err == nil {
		t.Fatal("expected script failure")
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Operation != "auth_login" || result.Script.ExitCode != 9 {
		t.Fatalf("expected structured auth_login error, got result=%#v err=%+v", result, err)
	}
	assertNoSecretLeak(t, result, "super-secret", "raw-token")
	assertNoSecretLeak(t, captureErr.Metadata, "super-secret", "raw-token")
}

func TestLoginAuthStateValidatesRequiredInputs(t *testing.T) {
	_, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       "http://localhost:9090",
		CookieJar:     "cookies.txt",
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
	}, time.Now())
	if err == nil {
		t.Fatal("expected missing password env error")
	}
	assertNoSecretLeak(t, err, "ADMIN_PASSWORD_VALUE")
}

func TestLoginAuthStateReportsMissingBash(t *testing.T) {
	original := authLookPath
	authLookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	defer func() {
		authLookPath = original
	}()
	t.Setenv("ADMIN_PASSWORD", "super-secret")

	_, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       "http://localhost:9090",
		CookieJar:     filepath.Join(t.TempDir(), "cookies.txt"),
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
	}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "bash") {
		t.Fatalf("expected missing bash error, got %v", err)
	}
	assertNoSecretLeak(t, err, "super-secret")
}

func TestLoginAuthStateEmbeddedScriptReportsMissingCurl(t *testing.T) {
	requireBash(t)
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	originalLookPath := authLookPath
	authLookPath = func(file string) (string, error) {
		if file == "bash" {
			return bashPath, nil
		}
		return originalLookPath(file)
	}
	defer func() {
		authLookPath = originalLookPath
	}()
	t.Setenv("PATH", "")
	t.Setenv("ADMIN_PASSWORD", "super-secret")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       "http://127.0.0.1:1",
		CookieJar:     filepath.Join(t.TempDir(), "cookies.txt"),
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		Timeout:       "5s",
	}, time.Now())
	if err == nil {
		t.Fatal("expected missing curl script failure")
	}
	if result.Script.ExitCode != 127 || !strings.Contains(result.Script.Stderr, "curl") {
		t.Fatalf("expected missing curl diagnostic, got %#v", result.Script)
	}
	assertNoSecretLeak(t, result, "super-secret")
}

func TestLoginAuthStateTimeout(t *testing.T) {
	requireBash(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "slow.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\nsleep 2\n"), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "super-secret")
	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    scriptPath,
		BaseURL:       "http://localhost:9090",
		CookieJar:     filepath.Join(dir, "cookies.txt"),
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		Timeout:       "10ms",
	}, time.Now())
	if err == nil || !result.Script.TimedOut {
		t.Fatalf("expected timeout, got result=%#v err=%v", result, err)
	}
}

func newAuthLoginTestServer(t *testing.T, csrfMode string) *httptest.Server {
	t.Helper()
	const csrfToken = "csrf-test-token"
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: "admin_debug_session", Value: "debug-secret", Path: "/"})
			if csrfMode == "header" {
				w.Header().Set("X-CSRF-Token", csrfToken)
			}
			_, _ = w.Write([]byte(`<form method="post"><input type="hidden" name="_token" value="` + csrfToken + `"></form>`))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			if r.Form.Get("identifier") != "admin" || r.Form.Get("password") != "correct-password" || r.Form.Get("_token") != csrfToken {
				http.Redirect(w, r, "/admin/login", http.StatusFound)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "admin_session", Value: "server-cookie-secret", Path: "/admin", Expires: time.Unix(1893456000, 0)})
			http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("admin_session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("dashboard"))
	})
	return httptest.NewServer(mux)
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

func requireCurl(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available")
	}
}

func TestRedactAuthSecrets(t *testing.T) {
	input := strings.Join([]string{
		"password=super-secret",
		"Authorization: Bearer raw-token",
		"Set-Cookie: admin_session=cookie-secret",
		"csrf=csrf-secret",
	}, "\n")
	redacted := RedactAuthSecrets(input, "super-secret", "cookie-secret")
	for _, secret := range []string{"super-secret", "cookie-secret", "raw-token", "csrf-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("secret %q leaked in %s", secret, redacted)
		}
	}
	if strings.TrimSpace(redacted) == "" {
		t.Fatal("redaction should retain non-secret diagnostic text")
	}
}

func TestDefaultGoAdminScriptEmbedded(t *testing.T) {
	script := defaultGoAdminScriptForTest()
	for _, expected := range []string{"x-csrf-token", "_token", "curl", "WEBCAP_PASSWORD"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("embedded script missing %q", expected)
		}
	}
}

func TestLoginAuthStateEmbeddedScriptInvalidCredentials(t *testing.T) {
	requireBash(t)
	requireCurl(t)
	server := newAuthLoginTestServer(t, "header")
	jarPath := filepath.Join(t.TempDir(), "cookies.txt")
	t.Setenv("ADMIN_PASSWORD", "wrong-password")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       server.URL,
		TargetURL:     server.URL + "/admin/dashboard",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		LoginPath:     DefaultAuthLoginPath,
		Timeout:       "10s",
	}, time.Unix(1700000000, 0))
	if err == nil {
		t.Fatal("expected invalid credentials failure")
	}
	if !strings.Contains(result.Script.Stderr, "login route") {
		t.Fatalf("expected login route diagnostic, got %#v", result.Script)
	}
	assertNoSecretLeak(t, result, "wrong-password")
}

func TestLoginAuthStateEnvironmentIncludesTargetURL(t *testing.T) {
	env := authLoginEnvironment(AuthLoginRequest{
		BaseURL:       "http://localhost:9090",
		LoginPath:     DefaultAuthLoginPath,
		TargetURL:     "http://localhost:9090/admin",
		CookieJar:     "cookies.txt",
		Identifier:    "admin",
		ExpectCookies: []string{"admin_session"},
	}, "secret")
	joined := strings.Join(env, "\n")
	for _, expected := range []string{"WEBCAP_TARGET_URL=http://localhost:9090/admin", "WEBCAP_EXPECT_COOKIE=admin_session"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected env %q in %s", expected, joined)
		}
	}
}

func TestLoginAuthStateEmbeddedScriptMissingCSRF(t *testing.T) {
	requireBash(t)
	requireCurl(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/login" {
			_, _ = w.Write([]byte(`<form method="post"></form>`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("ADMIN_PASSWORD", "correct-password")

	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		BaseURL:       server.URL,
		CookieJar:     filepath.Join(t.TempDir(), "cookies.txt"),
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
		Timeout:       "10s",
	}, time.Now())
	if err == nil {
		t.Fatal("expected missing CSRF failure")
	}
	if !strings.Contains(result.Script.Stderr, "CSRF") {
		t.Fatalf("expected CSRF diagnostic, got %#v", result.Script)
	}
}

func TestLoginAuthStateCustomScriptMissingExpectedCookie(t *testing.T) {
	requireBash(t)
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "cookies.txt")
	scriptPath := filepath.Join(dir, "write-debug.sh")
	script := `#!/usr/bin/env bash
cat > "$WEBCAP_COOKIE_JAR" <<COOKIEJAR
# Netscape HTTP Cookie File
localhost	FALSE	/	FALSE	1893456000	admin_debug_session	debug-cookie-secret
COOKIEJAR
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "super-secret")
	result, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    scriptPath,
		BaseURL:       "http://localhost:9090",
		TargetURL:     "http://localhost:9090/admin",
		CookieJar:     jarPath,
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
	}, time.Unix(1700000000, 0))
	if err == nil {
		t.Fatal("expected missing expected cookie failure")
	}
	if len(result.Inspection.Warnings) != 1 {
		t.Fatalf("expected debug-only warning, got %#v", result.Inspection)
	}
	assertNoSecretLeak(t, result, "debug-cookie-secret", "super-secret")
}

func TestLoginAuthStateRejectsUnreadableCustomScript(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "super-secret")
	_, err := LoginAuthState(context.Background(), AuthLoginRequest{
		ScriptPath:    filepath.Join(t.TempDir(), "missing.sh"),
		BaseURL:       "http://localhost:9090",
		CookieJar:     "cookies.txt",
		Identifier:    "admin",
		PasswordEnv:   "ADMIN_PASSWORD",
		ExpectCookies: []string{"admin_session"},
	}, time.Now())
	if err == nil {
		t.Fatal("expected unreadable script error")
	}
}

func TestEmbeddedLoginServerURLSanity(t *testing.T) {
	server := newAuthLoginTestServer(t, "header")
	parsed, err := url.Parse(server.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("unexpected server URL: %s", server.URL)
	}
}
