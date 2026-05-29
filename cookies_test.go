package webcap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCookieJarFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
	payload := strings.Join([]string{
		"# Netscape HTTP Cookie File",
		"localhost\tFALSE\t/\tFALSE\t0\tsession\tlocal-secret",
		"#HttpOnly_.example.test\tTRUE\t/admin\tTRUE\t1893456000\tadmin\tsecret-value",
		"expired.test\tFALSE\t/\tFALSE\t1\told\told-secret",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write cookie jar: %v", err)
	}

	cookies, err := ParseCookieJarFile(path, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("ParseCookieJarFile returned error: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("unexpected cookie count: %#v", cookies)
	}
	if cookies[0].Name != "session" || cookies[0].Domain != "localhost" || !cookies[0].HostOnly {
		t.Fatalf("unexpected localhost cookie: %#v", cookies[0])
	}
	if cookies[1].Name != "admin" || !cookies[1].HTTPOnly || !cookies[1].Secure || cookies[1].Path != "/admin" {
		t.Fatalf("unexpected httponly cookie: %#v", cookies[1])
	}
}

func TestParseCookieJarFileErrorsDoNotLeakValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(path, []byte("localhost\tFALSE\t/\tFALSE\tbad\tsecret\tleaky-value\n"), 0o644); err != nil {
		t.Fatalf("write cookie jar: %v", err)
	}

	err := func() error {
		_, err := ParseCookieJarFile(path, time.Now())
		return err
	}()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if strings.Contains(err.Error(), "leaky-value") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("parse error leaked cookie material: %v", err)
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Metadata["line"] != 1 {
		t.Fatalf("expected line metadata, got %+v", err)
	}
}

func TestLoadPlaywrightStorageStateDetectsOriginStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	payload := `{"cookies":[{"name":"sid","value":"secret","domain":"localhost","path":"/","expires":0}],"origins":[{"origin":"http://localhost:3000","localStorage":[{"name":"token","value":"secret"}]}]}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write storage state: %v", err)
	}
	state, err := LoadPlaywrightStorageState(path)
	if err != nil {
		t.Fatalf("LoadPlaywrightStorageState returned error: %v", err)
	}
	if len(state.Cookies) != 1 || state.Cookies[0].Name != "sid" {
		t.Fatalf("unexpected storage cookies: %#v", state.Cookies)
	}
	if !state.HasOriginStorage() {
		t.Fatal("expected origin storage to be detected")
	}
}
