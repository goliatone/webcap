package webcap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEngineChromium(t *testing.T) {
	engine, err := NewEngine(EngineConfig{EngineName: EngineChromium, Headless: true})
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}
	if engine.Name() != "chromium" {
		t.Fatalf("unexpected engine name: %s", engine.Name())
	}
}

func TestNewEnginePlaywright(t *testing.T) {
	engine, err := NewEngine(EngineConfig{EngineName: EnginePlaywright, PlaywrightBrowser: "firefox", Headless: true})
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}
	if engine.Name() != "playwright" {
		t.Fatalf("unexpected engine name: %s", engine.Name())
	}
}

func TestPlaywrightEngineCaptureBridge(t *testing.T) {
	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "capture.mjs")
	script := `const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const payload = JSON.parse(Buffer.concat(chunks).toString("utf8"));
if (payload.request.wait_for_function !== "window.__webcapReady") {
  throw new Error("missing wait_for_function");
}
process.stdout.write(JSON.stringify({
  artifact: {
    image_format: "png",
    mode: "viewport",
    url: "http://localhost:3000",
    viewport: { width: 800, height: 600, scale_factor: 1 }
  },
  browser: {
    engine: "playwright",
    product: "firefox/1.0",
    headless: true
  },
  timing: {
    navigation_started_at: "2026-03-30T00:00:00Z",
    captured_at: "2026-03-30T00:00:01Z",
    total_duration: "1s"
  },
  warnings: [],
  bytes_base64: Buffer.from("png").toString("base64")
}));`
	if writeErr := os.WriteFile(scriptPath, []byte(script), 0o644); writeErr != nil {
		t.Fatalf("WriteFile returned error: %v", writeErr)
	}

	engine, err := NewPlaywrightEngine(PlaywrightOptions{
		NodeBinary: nodeBinary,
		RuntimeDir: dir,
		ScriptPath: scriptPath,
	})
	if err != nil {
		t.Fatalf("NewPlaywrightEngine returned error: %v", err)
	}
	result, err := engine.Capture(context.Background(), CaptureRequest{
		URL:             "http://localhost:3000",
		WaitForFunction: "window.__webcapReady",
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if string(result.Artifact.Bytes) != "png" {
		t.Fatalf("unexpected payload: %q", string(result.Artifact.Bytes))
	}
	if result.Browser.Engine != "playwright" {
		t.Fatalf("unexpected browser engine: %s", result.Browser.Engine)
	}
}

func TestPlaywrightCaptureErrorMapping(t *testing.T) {
	err := playwrightCaptureError(`{"message":"wait_for_function did not become truthy before timeout","code":"timeout_error","operation":"wait_ready","metadata":{"wait":"wait_for_function"}}`, errors.New("exit status 1"))
	var captureErr *Error
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured timeout error, got %T", err)
	}
	if captureErr.Code != CodeTimeout || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected timeout error: %+v", captureErr)
	}
	if strings.Contains(captureErr.Message, "__distinctiveNeverReady") {
		t.Fatalf("timeout message leaked predicate source: %q", captureErr.Message)
	}

	err = playwrightCaptureError(`{"message":"wait_for_function predicate failed","code":"capture_error","operation":"wait_ready","metadata":{"wait":"wait_for_function"}}`, errors.New("exit status 1"))
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured predicate error, got %T", err)
	}
	if captureErr.Code != CodeCapture || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected predicate error: %+v", captureErr)
	}
	if strings.Contains(captureErr.Message, "distinctive thrown predicate source") {
		t.Fatalf("predicate error message leaked predicate source: %q", captureErr.Message)
	}

	err = playwrightCaptureError("Error: wait_for_function predicate failed", errors.New("exit status 1"))
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected generic playwright error, got %T", err)
	}
	if captureErr.Operation != "playwright_capture" || captureErr.Metadata["wait"] == "wait_for_function" {
		t.Fatalf("unstructured output should not be classified as wait_for_function: %+v", captureErr)
	}
}

func TestURLGuardVerification(t *testing.T) {
	if err := verifyURLGuards(CaptureGuards{ExpectURL: "/admin", FailOnURL: []string{"/login"}}, "http://localhost:3000/admin"); err != nil {
		t.Fatalf("verifyURLGuards returned error: %v", err)
	}
	outcomes, err := evaluateURLGuardOutcomes(CaptureGuards{ExpectURL: "/admin", FailOnURL: []string{"/login"}}, "http://localhost:3000/admin")
	if err != nil {
		t.Fatalf("evaluateURLGuardOutcomes returned error: %v", err)
	}
	if len(outcomes) != 2 || outcomes[0].Kind != "expect_url" || !outcomes[0].Matched || outcomes[0].Status != "passed" || outcomes[1].Matched || outcomes[1].Status != "passed" {
		t.Fatalf("unexpected URL guard outcomes: %#v", outcomes)
	}
	err = verifyURLGuards(CaptureGuards{ExpectURL: "/admin"}, "http://localhost:3000/login")
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Operation != "verify_url_guard" || captureErr.Metadata["expect_url"] != "/admin" {
		t.Fatalf("unexpected expect_url guard error: %+v", err)
	}
	err = verifyURLGuards(CaptureGuards{FailOnURL: []string{"/login"}}, "http://localhost:3000/login")
	if !errors.As(err, &captureErr) || captureErr.Operation != "verify_url_guard" || captureErr.Metadata["fail_on_url"] != "/login" {
		t.Fatalf("unexpected fail_on_url guard error: %+v", err)
	}
}

func TestSelectorGuardOutcomes(t *testing.T) {
	outcomes := selectorGuardOutcomes([]string{"form.login", ".unauthorized"}, "http://localhost:3000/admin", "")
	if len(outcomes) != 2 || outcomes[0].Matched || outcomes[0].Status != "passed" || outcomes[1].Matched || outcomes[1].Status != "passed" {
		t.Fatalf("unexpected passing selector outcomes: %#v", outcomes)
	}
	outcomes = selectorGuardOutcomes([]string{"form.login", ".unauthorized"}, "http://localhost:3000/login", ".unauthorized")
	if len(outcomes) != 2 || outcomes[1].Kind != "fail_on_selector" || !outcomes[1].Matched || outcomes[1].Status != "failed" {
		t.Fatalf("unexpected failing selector outcomes: %#v", outcomes)
	}
}

func TestChromiumStorageStateRejectsOriginStorageWithoutLeakingValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	payload := `{"cookies":[{"name":"sid","value":"cookie-secret","domain":"localhost","path":"/"}],"origins":[{"origin":"http://localhost:3000","localStorage":[{"name":"token","value":"local-storage-secret"}]}]}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write storage state: %v", err)
	}
	_, err := resolveChromiumStorageStateCookies(CaptureRequest{Auth: CaptureAuth{StorageState: path}})
	if err == nil {
		t.Fatal("expected unsupported storage state error")
	}
	if strings.Contains(err.Error(), "cookie-secret") || strings.Contains(err.Error(), "local-storage-secret") {
		t.Fatalf("storage state error leaked values: %v", err)
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Code != CodeUnsupported || captureErr.Metadata["engine"] != "chromium" {
		t.Fatalf("unexpected storage state error: %+v", err)
	}
}

func TestPlaywrightRuntimeWaitForFunctionPredicateForms(t *testing.T) {
	engine := newTestPlaywrightRuntimeEngine(t)
	tests := []struct {
		name      string
		predicate string
	}{
		{name: "expression", predicate: `window.__webcapReady === true`},
		{name: "callable", predicate: `() => window.__webcapReady === true`},
		{name: "async callable", predicate: `async () => window.__webcapReady === true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Capture(context.Background(), CaptureRequest{
				URL:             delayedReadyDataURL(),
				WaitForFunction: tt.predicate,
				JavaScript:      `if (!window.__webcapReady) throw new Error("not ready after wait_for_function")`,
				Timeout:         "5s",
			})
			if err != nil {
				skipIfPlaywrightRuntimeUnavailable(t, err)
				t.Fatalf("Capture returned error: %v", err)
			}
		})
	}
}

func TestPlaywrightRuntimeWaitForFunctionFailuresAreStructured(t *testing.T) {
	engine := newTestPlaywrightRuntimeEngine(t)
	tests := []struct {
		name      string
		predicate string
		wantCode  ErrorCode
	}{
		{name: "pending promise", predicate: `async () => new Promise(() => {})`, wantCode: CodeTimeout},
		{name: "sync hang", predicate: `() => { while (true) {} }`, wantCode: CodeTimeout},
		{name: "throws", predicate: `() => { throw new Error("distinctive thrown predicate source") }`, wantCode: CodeCapture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Capture(context.Background(), CaptureRequest{
				URL:             `data:text/html,<!doctype html><html><body>ready</body></html>`,
				WaitForFunction: tt.predicate,
				Timeout:         "1500ms",
			})
			if err != nil {
				skipIfPlaywrightRuntimeUnavailable(t, err)
			}
			var captureErr *Error
			if !errors.As(err, &captureErr) {
				t.Fatalf("expected structured error, got %T", err)
			}
			if captureErr.Code != tt.wantCode || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
				t.Fatalf("unexpected wait_for_function error: %+v", captureErr)
			}
			if strings.Contains(captureErr.Message, "distinctive thrown predicate source") {
				t.Fatalf("predicate source leaked into message: %q", captureErr.Message)
			}
		})
	}
}

func TestPlaywrightRuntimeWaitForFunctionModule(t *testing.T) {
	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	cmd := exec.Command(nodeBinary, "--test", "playwright_runtime/wait_for_function.test.mjs")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node runtime tests failed: %v\n%s", err, output)
	}
}

func TestPlaywrightEngineHardTimeoutClassifiesWaitForFunction(t *testing.T) {
	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "capture.mjs")
	script := `setInterval(() => {}, 1000);`
	if writeErr := os.WriteFile(scriptPath, []byte(script), 0o644); writeErr != nil {
		t.Fatalf("WriteFile returned error: %v", writeErr)
	}

	engine, err := NewPlaywrightEngine(PlaywrightOptions{
		NodeBinary: nodeBinary,
		RuntimeDir: dir,
		ScriptPath: scriptPath,
	})
	if err != nil {
		t.Fatalf("NewPlaywrightEngine returned error: %v", err)
	}
	_, err = engine.Capture(context.Background(), CaptureRequest{
		URL:             "http://localhost:3000",
		WaitForFunction: "() => { while (true) {} }",
		Timeout:         "25ms",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured timeout error, got %T", err)
	}
	if captureErr.Code != CodeTimeout || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected timeout error: %+v", captureErr)
	}
	if strings.Contains(captureErr.Message, "while (true)") {
		t.Fatalf("timeout message leaked predicate source: %q", captureErr.Message)
	}
}

func TestPlaywrightEngineRejectsTiledCapture(t *testing.T) {
	engine, err := NewPlaywrightEngine(PlaywrightOptions{
		NodeBinary: "node",
		RuntimeDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewPlaywrightEngine returned error: %v", err)
	}
	_, err = engine.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		OversizePolicy: OversizePolicyTile,
	})
	if err == nil {
		t.Fatal("expected unsupported tiled capture error")
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured error, got %T", err)
	}
	if captureErr.Code != CodeUnsupported || captureErr.Metadata["engine"] != "playwright" {
		t.Fatalf("unexpected unsupported error: %+v", captureErr)
	}
}

func newTestPlaywrightRuntimeEngine(t *testing.T) *PlaywrightEngine {
	t.Helper()
	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	runtimeDir := DefaultPlaywrightRuntimeDir()
	if _, err := os.Stat(filepath.Join(runtimeDir, "node_modules", "playwright")); err != nil {
		t.Skip("playwright runtime dependencies not installed")
	}
	engine, err := NewPlaywrightEngine(PlaywrightOptions{
		NodeBinary: nodeBinary,
		RuntimeDir: runtimeDir,
		Headless:   true,
	})
	if err != nil {
		t.Fatalf("NewPlaywrightEngine returned error: %v", err)
	}
	return engine
}

func skipIfPlaywrightRuntimeUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "executable doesn't exist") ||
		strings.Contains(message, "please run") ||
		strings.Contains(message, "browserType.launch") {
		t.Skipf("playwright browser unavailable: %v", err)
	}
}

func TestChromiumWaitForFunctionPredicateForms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="app">loading</div><script>
window.__webcapReady = false;
setTimeout(() => {
  window.__webcapReady = true;
  document.getElementById("app").textContent = "ready";
}, 50);
</script></body></html>`))
	}))
	defer server.Close()

	tests := []struct {
		name      string
		predicate string
	}{
		{name: "expression", predicate: `window.__webcapReady === true`},
		{name: "callable", predicate: `() => window.__webcapReady === true`},
		{name: "async callable", predicate: `async () => window.__webcapReady === true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewChromiumEngine(ChromiumOptions{Headless: true})
			_, err := engine.Capture(context.Background(), CaptureRequest{
				URL:             server.URL,
				WaitForFunction: tt.predicate,
				JavaScript:      `if (!window.__webcapReady) throw new Error("not ready after wait_for_function")`,
				Timeout:         "5s",
			})
			if err != nil {
				skipIfChromiumUnavailable(t, err)
				t.Fatalf("Capture returned error: %v", err)
			}
		})
	}
}

func delayedReadyDataURL() string {
	return `data:text/html,%3C%21doctype%20html%3E%3Chtml%3E%3Cbody%3E%3Cdiv%20id%3D%22app%22%3Eloading%3C%2Fdiv%3E%3Cscript%3Ewindow.__webcapReady%3Dfalse%3BsetTimeout%28%28%29%3D%3E%7Bwindow.__webcapReady%3Dtrue%3Bdocument.getElementById%28%22app%22%29.textContent%3D%22ready%22%3B%7D%2C50%29%3B%3C%2Fscript%3E%3C%2Fbody%3E%3C%2Fhtml%3E`
}

func TestChromiumWaitForFunctionFailuresAreStructuredAndRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body>ready</body></html>`))
	}))
	defer server.Close()

	engine := NewChromiumEngine(ChromiumOptions{Headless: true})
	_, err := engine.Capture(context.Background(), CaptureRequest{
		URL:             server.URL,
		WaitForFunction: `window.__distinctiveNeverReadyPredicate === true`,
		Timeout:         "250ms",
	})
	if err != nil {
		skipIfChromiumUnavailable(t, err)
	}
	var captureErr *Error
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured timeout error, got %T", err)
	}
	if captureErr.Code != CodeTimeout || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected timeout error: %+v", captureErr)
	}
	if strings.Contains(captureErr.Message, "__distinctiveNeverReadyPredicate") {
		t.Fatalf("timeout message leaked predicate source: %q", captureErr.Message)
	}

	_, err = engine.Capture(context.Background(), CaptureRequest{
		URL:             server.URL,
		WaitForFunction: `() => { throw new Error("distinctive thrown predicate source") }`,
		Timeout:         "5s",
	})
	if err != nil {
		skipIfChromiumUnavailable(t, err)
	}
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured thrown predicate error, got %T", err)
	}
	if captureErr.Code != CodeCapture || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected predicate error: %+v", captureErr)
	}
	if strings.Contains(captureErr.Message, "distinctive thrown predicate source") {
		t.Fatalf("predicate error message leaked predicate source: %q", captureErr.Message)
	}

	_, err = engine.Capture(context.Background(), CaptureRequest{
		URL:             server.URL,
		WaitForFunction: `async () => new Promise(() => {})`,
		Timeout:         "3s",
	})
	if err != nil {
		skipIfChromiumUnavailable(t, err)
	}
	if !errors.As(err, &captureErr) {
		t.Fatalf("expected structured pending predicate timeout, got %T", err)
	}
	if captureErr.Code != CodeTimeout || captureErr.Operation != "wait_ready" || captureErr.Metadata["wait"] != "wait_for_function" {
		t.Fatalf("unexpected pending predicate timeout: %+v", captureErr)
	}
}

func skipIfChromiumUnavailable(t *testing.T, err error) {
	t.Helper()
	var captureErr *Error
	if errors.As(err, &captureErr) && captureErr.Code == CodeBrowserStartup {
		t.Skipf("chromium unavailable: %v", err)
	}
}
