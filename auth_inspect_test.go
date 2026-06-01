package webcap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInspectAuthStateCookieJarExpectedCookieApplicable(t *testing.T) {
	now := time.Unix(1700000000, 0)
	path := writeCookieJar(t, []string{
		"localhost\tFALSE\t/admin\tFALSE\t1893456000\tadmin_session\tsecret-value",
	})

	result, err := InspectAuthState(AuthInspectRequest{
		CookieJar:     path,
		TargetURL:     "http://localhost:9090/admin/translations/queue",
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err != nil {
		t.Fatalf("InspectAuthState returned error: %v", err)
	}
	if result.Source.Type != authSourceCookieJar || len(result.Cookies) != 1 {
		t.Fatalf("unexpected source/cookies: %#v", result)
	}
	if result.Cookies[0].Name != "admin_session" || result.Cookies[0].Applicable == nil || !*result.Cookies[0].Applicable {
		t.Fatalf("expected applicable cookie diagnostic: %#v", result.Cookies[0])
	}
	if len(result.ExpectedCookies) != 1 || result.ExpectedCookies[0].Status != authCookieStatusPresent {
		t.Fatalf("unexpected expected cookie status: %#v", result.ExpectedCookies)
	}
	assertNoSecretLeak(t, result, "secret-value")
}

func TestInspectAuthStateReportsExpiredExpectedCookieNotMissing(t *testing.T) {
	now := time.Unix(1700000000, 0)
	path := writeCookieJar(t, []string{
		"localhost\tFALSE\t/\tFALSE\t1\tadmin_session\told-secret",
	})

	result, err := InspectAuthState(AuthInspectRequest{
		CookieJar:     path,
		TargetURL:     "http://localhost:9090/admin",
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err == nil {
		t.Fatal("expected expired cookie validation error")
	}
	if len(result.ExpectedCookies) != 1 || result.ExpectedCookies[0].Status != authCookieStatusExpired {
		t.Fatalf("expected expired status, got %#v", result.ExpectedCookies)
	}
	if len(result.Cookies) != 1 || !result.Cookies[0].Expired || result.Cookies[0].CaptureImportOK {
		t.Fatalf("expected expired cookie diagnostic: %#v", result.Cookies)
	}

	imported, importErr := ParseCookieJarFile(path, now)
	if importErr != nil {
		t.Fatalf("ParseCookieJarFile returned error: %v", importErr)
	}
	if len(imported) != 0 {
		t.Fatalf("capture-time parser should still skip expired cookies: %#v", imported)
	}
	assertNoSecretLeak(t, err, "old-secret")
	assertNoSecretLeak(t, result, "old-secret")
}

func TestInspectAuthStateCookieApplicabilityFailures(t *testing.T) {
	now := time.Unix(1700000000, 0)
	path := writeCookieJar(t, []string{
		"admin.local\tFALSE\t/admin\tTRUE\t1893456000\tadmin_session\tsecret-value",
	})

	result, err := InspectAuthState(AuthInspectRequest{
		CookieJar:     path,
		TargetURL:     "http://localhost:9090/settings",
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err == nil {
		t.Fatal("expected not-applicable validation error")
	}
	if len(result.ExpectedCookies) != 1 || result.ExpectedCookies[0].Status != authCookieStatusNotApplicable {
		t.Fatalf("unexpected expected status: %#v", result.ExpectedCookies)
	}
	reasons := strings.Join(result.ExpectedCookies[0].Reasons, ",")
	for _, expected := range []string{"domain_mismatch", "path_mismatch", "secure_cookie_on_http"} {
		if !strings.Contains(reasons, expected) {
			t.Fatalf("expected reason %q in %#v", expected, result.ExpectedCookies[0].Reasons)
		}
	}
}

func TestInspectAuthStateWarnsAboutDebugOnlyCookieJar(t *testing.T) {
	now := time.Unix(1700000000, 0)
	path := writeCookieJar(t, []string{
		"localhost\tFALSE\t/\tFALSE\t1893456000\tadmin_debug_session\tdebug-secret",
	})

	result, err := InspectAuthState(AuthInspectRequest{
		CookieJar:     path,
		TargetURL:     "http://localhost:9090/admin",
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err == nil {
		t.Fatal("expected missing auth cookie validation error")
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "debug_cookie_without_auth" {
		t.Fatalf("expected debug-only warning: %#v", result.Warnings)
	}
	assertNoSecretLeak(t, result, "debug-secret")
}

func TestInspectAuthStateStorageStateCookieDiagnosticsDoNotLeakStorageValues(t *testing.T) {
	now := time.Unix(1700000000, 0)
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	payload := `{"cookies":[{"name":"admin_session","value":"cookie-secret","domain":"localhost","path":"/admin","expires":1893456000}],"origins":[{"origin":"http://localhost:9090","localStorage":[{"name":"token","value":"storage-secret"}]}]}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write storage state: %v", err)
	}

	result, err := InspectAuthState(AuthInspectRequest{
		StorageState:  path,
		TargetURL:     "http://localhost:9090/admin/translations/queue",
		ExpectCookies: []string{"admin_session"},
	}, now)
	if err != nil {
		t.Fatalf("InspectAuthState returned error: %v", err)
	}
	if !result.HasOriginStorage || len(result.Cookies) != 1 || !result.Cookies[0].StorageState {
		t.Fatalf("unexpected storage-state diagnostics: %#v", result)
	}
	assertNoSecretLeak(t, result, "cookie-secret", "storage-secret")
}

func TestInspectAuthStateErrorDoesNotLeakCookieJarValues(t *testing.T) {
	path := writeCookieJar(t, []string{
		"localhost\tFALSE\t/\tFALSE\tbad\tadmin_session\tsecret-value",
	})
	_, err := InspectAuthState(AuthInspectRequest{CookieJar: path}, time.Now())
	if err == nil {
		t.Fatal("expected parse error")
	}
	assertNoSecretLeak(t, err, "secret-value")
	var captureErr *Error
	if !errors.As(err, &captureErr) || strings.TrimSpace(toString(captureErr.Metadata["line"])) != "2" {
		t.Fatalf("expected line metadata, got %+v", err)
	}
}

func writeCookieJar(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
	payload := "# Netscape HTTP Cookie File\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write cookie jar: %v", err)
	}
	return path
}

func assertNoSecretLeak(t *testing.T, value any, secrets ...string) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte(strings.TrimSpace(toString(value)))
	}
	text := string(payload)
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %q leaked in %s", secret, text)
		}
	}
}

func toString(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	return fmt.Sprint(value)
}
