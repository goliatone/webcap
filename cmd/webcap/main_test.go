package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/version"
)

func TestRunHelpCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"help"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	for _, expected := range []string{
		"Usage:",
		"webcap help",
		"webcap version",
		"webcap shot [flags] <url>",
		"webcap mcp serve [flags]",
		"webcap skill install [flags] --agent <codex|claude>",
		"--wait-for-function",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunVersionCLI(t *testing.T) {
	oldTag, oldTime, oldUser, oldCommit := version.Tag, version.Time, version.User, version.Commit
	t.Cleanup(func() {
		version.Tag = oldTag
		version.Time = oldTime
		version.User = oldUser
		version.Commit = oldCommit
	})
	version.Tag = "0.1.0"
	version.Time = "2026-05-08T00:00:00Z"
	version.User = "builder"
	version.Commit = "abc123"

	invocation, err := parseCLI([]string{"version"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	for _, expected := range []string{
		"Version:",
		"0.1.0",
		"Build Commit Hash:",
		"abc123",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected version output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunSkillInstallUsesEmbeddedAssets(t *testing.T) {
	tests := []struct {
		agent       string
		destination func(string) string
	}{
		{
			agent: "codex",
			destination: func(home string) string {
				return filepath.Join(home, ".agents", "skills", "webcap-agent")
			},
		},
		{
			agent: "claude",
			destination: func(home string) string {
				return filepath.Join(home, ".claude", "skills", "webcap-agent")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			invocation, err := parseCLI([]string{"skill", "install", "--agent", tt.agent})
			if err != nil {
				t.Fatalf("parseCLI returned error: %v", err)
			}

			var runErr error
			output := captureStdout(t, func() {
				runErr = run(context.Background(), invocation)
			})
			if runErr != nil {
				t.Fatalf("run returned error: %v", runErr)
			}
			destination := tt.destination(home)
			for _, expected := range []string{
				"Skill install complete",
				"Agent: " + tt.agent,
				"Skill: webcap-agent",
				destination,
			} {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected skill install output to contain %q, got:\n%s", expected, output)
				}
			}
			for _, path := range []string{
				filepath.Join(destination, "SKILL.md"),
				filepath.Join(destination, "references", "cli.md"),
				filepath.Join(destination, "references", "visual-review.md"),
			} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("expected installed asset %s: %v", path, err)
				}
			}
		})
	}
}

func TestRunSkillInstallRejectsConflictUnlessForced(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	destination := filepath.Join(home, ".agents", "skills", "webcap-agent")

	invocation, err := parseCLI([]string{"skill", "install", "--agent", "codex"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	var runErr error
	_ = captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("initial run returned error: %v", runErr)
	}

	skillPath := filepath.Join(destination, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# User edit\n"), 0o644); err != nil {
		t.Fatalf("write conflicting skill: %v", err)
	}

	_ = captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "--force") {
		t.Fatalf("expected conflict error mentioning --force, got %v", runErr)
	}
	got, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read conflicting skill: %v", err)
	}
	if string(got) != "# User edit\n" {
		t.Fatalf("conflict install changed existing file: %q", got)
	}

	forced, err := parseCLI([]string{"skill", "install", "--agent", "codex", "--force"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	output := captureStdout(t, func() {
		runErr = run(context.Background(), forced)
	})
	if runErr != nil {
		t.Fatalf("forced run returned error: %v", runErr)
	}
	if !strings.Contains(output, "Files written:") {
		t.Fatalf("expected forced install output to include files_written, got:\n%s", output)
	}
	got, err = os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read forced skill: %v", err)
	}
	if string(got) == "# User edit\n" {
		t.Fatal("forced install did not replace conflicting file")
	}
}

func TestRunSkillInstallJSONModePreservesResultShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	invocation, err := parseCLI([]string{"skill", "install", "--json", "--agent", "codex"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	var decoded struct {
		Agent        string `json:"agent"`
		SkillName    string `json:"skill_name"`
		Destination  string `json:"destination"`
		FilesWritten int    `json:"files_written"`
	}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if decoded.Agent != "codex" || decoded.SkillName != "webcap-agent" || decoded.FilesWritten == 0 {
		t.Fatalf("unexpected install result: %#v", decoded)
	}
}

func TestRunSkillInstallJSONConflictWritesStructuredEnvelope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	args := []string{"skill", "install", "--agent", "codex"}
	var stdout, stderr bytes.Buffer
	if code := runCLI(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("initial install failed with code %d, stderr:\n%s", code, stderr.String())
	}
	skillPath := filepath.Join(home, ".agents", "skills", "webcap-agent", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# User edit\n"), 0o644); err != nil {
		t.Fatalf("write conflicting skill: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code := runCLI(context.Background(), []string{"skill", "install", "--json", "--agent", "codex"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected conflict to exit non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got:\n%s", stdout.String())
	}
	var envelope struct {
		Message   string         `json:"message"`
		Code      string         `json:"code"`
		Operation string         `json:"operation"`
		Metadata  map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, stderr.String())
	}
	if envelope.Code != "skill_conflict" || envelope.Operation != "skill_install" || envelope.Metadata["path"] != skillPath {
		t.Fatalf("unexpected conflict envelope: %#v", envelope)
	}
	if !strings.Contains(envelope.Message, "--force") {
		t.Fatalf("expected conflict message to mention --force, got %#v", envelope)
	}
}

func TestRunMultiJSONModePreservesBatchShapeWithFakeService(t *testing.T) {
	invocation, err := parseCLI([]string{"multi", "--json", "manifest.yaml"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{
		batch: pkgwebcap.BatchResult{Results: []pkgwebcap.CaptureResult{{
			OutputPath:   "shots/home.png",
			MetadataPath: "shots/home.png.json",
			ByteSize:     42,
		}}},
	}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.loadManifest = func(path string) (pkgwebcap.Manifest, error) {
		if path != "manifest.yaml" {
			t.Fatalf("unexpected manifest path: %s", path)
		}
		return pkgwebcap.Manifest{}, nil
	}
	app.newCaptureService = func(browserOptions, semanticProviderOptions) (cliService, error) {
		return fake, nil
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var decoded struct {
		Results []struct {
			OutputPath   string `json:"output_path"`
			MetadataPath string `json:"metadata_path"`
			ByteSize     int    `json:"byte_size"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(decoded.Results) != 1 || decoded.Results[0].OutputPath != "shots/home.png" || decoded.Results[0].MetadataPath != "shots/home.png.json" || decoded.Results[0].ByteSize != 42 {
		t.Fatalf("unexpected batch JSON: %#v", decoded)
	}
}

func TestRunShotPassesFullPageDefaultToService(t *testing.T) {
	invocation, err := parseCLI([]string{"shot", "--json", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{
		capture: pkgwebcap.CaptureResult{
			OutputPath: "shots/home.png",
			Artifact:   pkgwebcap.CaptureArtifact{Mode: pkgwebcap.CaptureModeFullPage},
		},
	}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.newCaptureService = func(browserOptions, semanticProviderOptions) (cliService, error) {
		return fake, nil
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !fake.lastCaptureRequest.FullPage {
		t.Fatalf("expected default full-page request, got %#v", fake.lastCaptureRequest)
	}
}

func TestRunShotPassesVisibleModeToService(t *testing.T) {
	invocation, err := parseCLI([]string{"shot", "--json", "--visible", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{capture: pkgwebcap.CaptureResult{OutputPath: "shots/home.png"}}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.newCaptureService = func(browserOptions, semanticProviderOptions) (cliService, error) {
		return fake, nil
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if fake.lastCaptureRequest.FullPage {
		t.Fatalf("expected visible viewport request, got %#v", fake.lastCaptureRequest)
	}
}

func TestRunDiffJSONModePreservesDiffShapeWithFakeService(t *testing.T) {
	invocation, err := parseCLI([]string{"diff", "--json", "base.png", "current.png"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{
		diff: pkgwebcap.DiffResult{
			Mode:         pkgwebcap.DiffModeImage,
			OutputPath:   "diffs/home.png",
			MetadataPath: "diffs/home.png.json",
			Summary:      pkgwebcap.DiffSummary{ChangedFiles: 1, TotalChangedPixels: 9},
		},
	}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.newService = func(semanticProviderOptions) cliService {
		return fake
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var decoded struct {
		Mode         string `json:"mode"`
		OutputPath   string `json:"output_path"`
		MetadataPath string `json:"metadata_path"`
		Summary      struct {
			ChangedFiles       int `json:"changed_files"`
			TotalChangedPixels int `json:"total_changed_pixels"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.OutputPath != "diffs/home.png" || decoded.MetadataPath != "diffs/home.png.json" || decoded.Summary.ChangedFiles != 1 || decoded.Summary.TotalChangedPixels != 9 {
		t.Fatalf("unexpected diff JSON: %#v", decoded)
	}
}

func TestRunWorkflowRunReportJSONModePreservesShapeWithFakeService(t *testing.T) {
	invocation, err := parseCLI([]string{"workflow", "capture-scenario", "--json", "--run-report", "workflow.yaml"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{
		workflowCapture: pkgwebcap.WorkflowCaptureResult{
			ScenarioID: "checkout",
			Results:    []pkgwebcap.WorkflowScreenCaptureResult{{ScreenID: "home", OutputPath: "current/home.png"}},
		},
		workflowReport: pkgwebcap.WorkflowReportResult{
			ScenarioID:   "checkout",
			ReportPath:   "reports/index.html",
			MetadataPath: "reports/report.json",
			Status:       pkgwebcap.WorkflowReviewStatus{Level: "info", Label: "Needs Review"},
		},
	}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.loadScenario = func(path string) (pkgwebcap.WorkflowScenario, error) {
		if path != "workflow.yaml" {
			t.Fatalf("unexpected scenario path: %s", path)
		}
		return pkgwebcap.WorkflowScenario{ID: "checkout"}, nil
	}
	app.newScenarioService = func(browserOptions, semanticProviderOptions, pkgwebcap.WorkflowScenario) (cliService, error) {
		return fake, nil
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var decoded struct {
		Capture struct {
			ScenarioID string `json:"scenario_id"`
			Results    []struct {
				ScreenID string `json:"screen_id"`
			} `json:"results"`
		} `json:"capture"`
		Report struct {
			ReportPath   string `json:"report_path"`
			MetadataPath string `json:"metadata_path"`
			Status       struct {
				Level string `json:"level"`
				Label string `json:"label"`
			} `json:"status"`
		} `json:"report"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.Capture.ScenarioID != "checkout" || len(decoded.Capture.Results) != 1 || decoded.Report.ReportPath != "reports/index.html" || decoded.Report.Status.Label != "Needs Review" {
		t.Fatalf("unexpected workflow run-report JSON: %#v", decoded)
	}
}

func TestRunReportJSONModePreservesReportShapeWithFakeService(t *testing.T) {
	invocation, err := parseCLI([]string{"report", "scenario", "--json", "workflow.yaml"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	fake := &fakeCLIService{
		workflowReport: pkgwebcap.WorkflowReportResult{
			ScenarioID:   "checkout",
			ReportPath:   "reports/index.html",
			MetadataPath: "reports/report.json",
			Status:       pkgwebcap.WorkflowReviewStatus{Level: "success", Label: "Ready"},
		},
	}
	var stdout, stderr bytes.Buffer
	app := newApp(strings.NewReader(""), &stdout, &stderr)
	app.loadScenario = func(path string) (pkgwebcap.WorkflowScenario, error) {
		if path != "workflow.yaml" {
			t.Fatalf("unexpected scenario path: %s", path)
		}
		return pkgwebcap.WorkflowScenario{ID: "checkout"}, nil
	}
	app.newService = func(semanticProviderOptions) cliService {
		return fake
	}
	if err := app.run(context.Background(), invocation); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var decoded struct {
		ScenarioID   string `json:"scenario_id"`
		ReportPath   string `json:"report_path"`
		MetadataPath string `json:"metadata_path"`
		Status       struct {
			Level string `json:"level"`
			Label string `json:"label"`
		} `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.ScenarioID != "checkout" || decoded.ReportPath != "reports/index.html" || decoded.MetadataPath != "reports/report.json" || decoded.Status.Label != "Ready" {
		t.Fatalf("unexpected report JSON: %#v", decoded)
	}
}

func TestRunCLIJSONParseErrorWritesEnvelopeToStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"shot", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got:\n%s", stdout.String())
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(envelope.Message, "shot requires exactly one positional url argument") {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestRunCLIExplicitJSONInvalidFormatWritesEnvelopeToStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"shot", "--json", "--format", "xml", "https://example.test"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(envelope.Message, `unsupported output format "xml"`) {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestRunCLIHumanSetupErrorWritesStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(context.Background(), []string{"shot", "--engine", "bogus", "https://example.test"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error: unsupported engine") {
		t.Fatalf("unexpected stderr:\n%s", stderr.String())
	}
}

func TestRunPresenterWriteErrorIsReturned(t *testing.T) {
	invocation, err := parseCLI([]string{"skill", "install", "--agent", "codex"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	runErr := newApp(strings.NewReader(""), errWriter{}, io.Discard).run(context.Background(), invocation)
	if !errors.Is(runErr, errWriteFailed) {
		t.Fatalf("expected writer error, got %v", runErr)
	}
}

func TestRunSemanticDiffUsesDefaultBuiltInProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected OpenAI auth header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"CLI semantic ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}]}]}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMainTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMainTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})
	t.Setenv("OPENAI_API_KEY", "test-key")

	invocation, err := parseCLI([]string{
		"semantic-diff",
		"--provider", "openai",
		"--model", "gpt-test",
		"--openai-base-url", server.URL,
		"--metadata", filepath.Join(dir, "semantic.json"),
		currentPath,
		referencePath,
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	if !strings.Contains(output, "CLI semantic ok") {
		t.Fatalf("expected semantic summary in output, got:\n%s", output)
	}
}

func TestRunSemanticDiffUsesCodexCLIProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMainTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMainTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})
	fake := writeMainFakeCodex(t, dir, `#!/bin/sh
cat >/dev/null
printf '{"summary":"CLI codex ok","verdict":"no_meaningful_change","severity":"info"}\n'
`)

	invocation, err := parseCLI([]string{
		"semantic-diff",
		"--provider", "codex-cli",
		"--model", "gpt-test",
		"--codex-bin", fake,
		"--metadata", filepath.Join(dir, "semantic.json"),
		currentPath,
		referencePath,
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	var runErr error
	output := captureStdout(t, func() {
		runErr = run(context.Background(), invocation)
	})
	if runErr != nil {
		t.Fatalf("run returned error: %v", runErr)
	}
	if !strings.Contains(output, "CLI codex ok") {
		t.Fatalf("expected codex semantic summary in output, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() {
		os.Stdout = original
		_ = reader.Close()
	}()

	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}

var errWriteFailed = errors.New("write failed")

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, errWriteFailed
}

func writeMainTestPNG(t *testing.T, path string, c color.NRGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}
}

func writeMainFakeCodex(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}

type fakeCLIService struct {
	capture            pkgwebcap.CaptureResult
	batch              pkgwebcap.BatchResult
	diff               pkgwebcap.DiffResult
	semantic           pkgwebcap.SemanticDiffResult
	workflowCapture    pkgwebcap.WorkflowCaptureResult
	workflowReport     pkgwebcap.WorkflowReportResult
	lastCaptureRequest pkgwebcap.CaptureRequest
}

func (f *fakeCLIService) CaptureArtifact(context.Context, pkgwebcap.CaptureRequest) (pkgwebcap.CaptureArtifact, error) {
	return f.capture.Artifact, nil
}

func (f *fakeCLIService) Capture(_ context.Context, req pkgwebcap.CaptureRequest) (pkgwebcap.CaptureResult, error) {
	f.lastCaptureRequest = req
	return f.capture, nil
}

func (f *fakeCLIService) CaptureBatch(context.Context, pkgwebcap.Manifest, string) (pkgwebcap.BatchResult, error) {
	return f.batch, nil
}

func (f *fakeCLIService) Diff(context.Context, pkgwebcap.DiffRequest) (pkgwebcap.DiffResult, error) {
	return f.diff, nil
}

func (f *fakeCLIService) SemanticDiff(context.Context, pkgwebcap.SemanticDiffRequest) (pkgwebcap.SemanticDiffResult, error) {
	return f.semantic, nil
}

func (f *fakeCLIService) CaptureScenario(context.Context, pkgwebcap.WorkflowScenario) (pkgwebcap.WorkflowCaptureResult, error) {
	return f.workflowCapture, nil
}

func (f *fakeCLIService) GenerateWorkflowReport(context.Context, pkgwebcap.WorkflowReportRequest) (pkgwebcap.WorkflowReportResult, error) {
	return f.workflowReport, nil
}
