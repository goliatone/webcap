package webcap

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/webcap/pkg/llms"
)

type recordingEngine struct {
	requests []CaptureRequest
	result   EngineResult
	err      error
}

func (e *recordingEngine) Name() string { return "recording" }

func (e *recordingEngine) Capture(ctx context.Context, req CaptureRequest) (EngineResult, error) {
	e.requests = append(e.requests, req)
	return e.result, e.err
}

func TestParseWorkflowStories(t *testing.T) {
	tempDir := t.TempDir()
	storyPath := filepath.Join(tempDir, "stories.md")
	writeWorkflowStoryFixture(t, storyPath)

	stories, err := parseWorkflowStories(storyPath)
	if err != nil {
		t.Fatalf("parseWorkflowStories returned error: %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("unexpected story count: %d", len(stories))
	}
	if stories["D1"].Priority != "P1" {
		t.Fatalf("unexpected D1 priority: %s", stories["D1"].Priority)
	}
	if !strings.Contains(stories["D1"].Title, "review a captured page") {
		t.Fatalf("unexpected D1 title: %s", stories["D1"].Title)
	}
	if len(stories["D1"].AcceptanceCriteria) == 0 {
		t.Fatal("expected D1 acceptance criteria")
	}
}

func TestLoadWorkflowScenarioWithOptions(t *testing.T) {
	tempDir := t.TempDir()
	scenarioPath := writeWorkflowScenarioFixture(t, tempDir)

	scenario, err := LoadWorkflowScenarioWithOptions(scenarioPath, WorkflowOptions{
		DefaultSelectedScenario: "fixture-scenario",
		DefaultPresentationMode: "review",
		HandoffQueryParam:       "fixture_handoff",
		BuildHandoff:            DefaultWorkflowHandoff,
	})
	if err != nil {
		t.Fatalf("LoadWorkflowScenario returned error: %v", err)
	}
	if scenario.ID != "fixture-flow" {
		t.Fatalf("unexpected scenario id: %s", scenario.ID)
	}
	if scenario.Environment.BaseURL != "http://localhost:8383" {
		t.Fatalf("unexpected base url: %s", scenario.Environment.BaseURL)
	}
	if scenario.Environment.SelectedScenario != "fixture-scenario" {
		t.Fatalf("unexpected selected scenario: %s", scenario.Environment.SelectedScenario)
	}
	if scenario.Environment.PresentationMode != "review" {
		t.Fatalf("unexpected presentation mode: %s", scenario.Environment.PresentationMode)
	}
	if len(scenario.Screens) != 1 {
		t.Fatalf("unexpected screen count: %d", len(scenario.Screens))
	}
	if scenario.Defaults.Viewport.Width != 1024 {
		t.Fatalf("unexpected default viewport: %+v", scenario.Defaults.Viewport)
	}
	if scenario.Artifacts.Root != filepath.Join(tempDir, "artifacts") {
		t.Fatalf("unexpected artifact root: %s", scenario.Artifacts.Root)
	}
	if _, ok := scenario.Stories["D1"]; !ok {
		t.Fatal("expected D1 story to be loaded from story source")
	}
	if !filepath.IsAbs(scenario.Screens[0].ReferenceImage) {
		t.Fatalf("expected absolute reference path, got %s", scenario.Screens[0].ReferenceImage)
	}
}

func TestCaptureScenarioBuildsRequestsWithConfiguredWorkflowOptions(t *testing.T) {
	tempDir := t.TempDir()
	scenarioPath := writeWorkflowScenarioFixture(t, tempDir)
	opts := WorkflowOptions{
		DefaultSelectedScenario: "fixture-scenario",
		DefaultPresentationMode: "review",
		HandoffQueryParam:       "fixture_handoff",
		BuildHandoff:            DefaultWorkflowHandoff,
	}
	scenario, err := LoadWorkflowScenarioWithOptions(scenarioPath, opts)
	if err != nil {
		t.Fatalf("LoadWorkflowScenario returned error: %v", err)
	}

	engine := &recordingEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:8383/triage/patient",
				Viewport:    Viewport{Width: 1024, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{Engine: "recording", Headless: true},
			Timing: CaptureTiming{
				NavigationStartedAt:   time.Unix(100, 0).UTC(),
				NavigationCompletedAt: time.Unix(101, 0).UTC(),
				ReadyAt:               time.Unix(102, 0).UTC(),
				CapturedAt:            time.Unix(103, 0).UTC(),
				TotalDuration:         "3s",
			},
		},
	}
	service := NewServiceWithOptions(engine, Options{Workflow: opts})

	result, err := service.CaptureScenario(context.Background(), scenario)
	if err != nil {
		t.Fatalf("CaptureScenario returned error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("unexpected capture count: %d", len(result.Results))
	}
	if len(engine.requests) != 1 {
		t.Fatalf("unexpected request count: %d", len(engine.requests))
	}

	first := engine.requests[0]
	if first.OutputPath != filepath.Join(scenario.Artifacts.CurrentDir, "patient.png") {
		t.Fatalf("unexpected output path: %s", first.OutputPath)
	}
	if !first.FullPage {
		t.Fatal("expected workflow requests to default to full-page capture")
	}
	parsedURL, err := url.Parse(first.URL)
	if err != nil {
		t.Fatalf("parse request url: %v", err)
	}
	query := parsedURL.Query()
	if query.Get("scenario") != "fixture-scenario" {
		t.Fatalf("unexpected scenario query: %s", query.Get("scenario"))
	}
	if query.Get("presentation_mode") != "review" {
		t.Fatalf("unexpected presentation mode: %s", query.Get("presentation_mode"))
	}
	if strings.TrimSpace(query.Get("fixture_handoff")) == "" {
		t.Fatal("expected configured handoff query")
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.CurrentDir, "patient.png.json")); err != nil {
		t.Fatalf("expected metadata sidecar to be written: %v", err)
	}
}

func TestCaptureScenarioCompilesHookScriptsInOrder(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("writeTestPNG: %v", err)
	}

	scenario := WorkflowScenario{
		ID:          "hook-order",
		Label:       "Hook Order",
		SourceDir:   tempDir,
		SourcePath:  filepath.Join(tempDir, "hook-order.yaml"),
		Stories:     map[string]WorkflowStory{"D1": {ID: "D1", Title: "Hook story"}},
		Environment: WorkflowEnvironment{BaseURL: "http://localhost:8383"},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Hooks: WorkflowHooks{
			AuthSetup: WorkflowHookSet{
				BeforeNavigate: "window.__order = (window.__order || []); window.__order.push('scenario-auth');",
			},
			Navigation: WorkflowHookSet{
				BeforeCapture: "window.__order = (window.__order || []); window.__order.push('scenario-nav-capture');",
			},
		},
		Screens: []WorkflowScreen{
			{
				ID:             "hooked",
				Route:          "/triage/patient",
				ReferenceImage: referencePath,
				PrimaryStories: []string{"D1"},
				Hooks: WorkflowHooks{
					StateSetup: WorkflowHookSet{
						BeforeNavigate: "window.__order.push('screen-state');",
					},
					Navigation: WorkflowHookSet{
						Mode:          WorkflowHookModeReplace,
						BeforeCapture: "window.__order.push('screen-nav-capture');",
					},
				},
			},
		},
	}

	engine := &recordingEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:8383/triage/patient",
				Viewport:    Viewport{Width: 1024, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{Engine: "recording", Headless: true},
			Timing: CaptureTiming{
				NavigationStartedAt:   time.Unix(100, 0).UTC(),
				NavigationCompletedAt: time.Unix(101, 0).UTC(),
				ReadyAt:               time.Unix(102, 0).UTC(),
				CapturedAt:            time.Unix(103, 0).UTC(),
				TotalDuration:         "3s",
			},
		},
	}
	service := NewService(engine)

	if _, err := service.CaptureScenario(context.Background(), scenario); err != nil {
		t.Fatalf("CaptureScenario returned error: %v", err)
	}
	if len(engine.requests) != 1 {
		t.Fatalf("unexpected request count: %d", len(engine.requests))
	}

	beforeNavigate := engine.requests[0].BeforeNavigateJS
	if !strings.Contains(beforeNavigate, "scenario-auth") || !strings.Contains(beforeNavigate, "screen-state") {
		t.Fatalf("expected compiled before navigate hook script, got %s", beforeNavigate)
	}
	if strings.Index(beforeNavigate, "scenario-auth") > strings.Index(beforeNavigate, "screen-state") {
		t.Fatalf("expected scenario hook before screen hook, got %s", beforeNavigate)
	}

	beforeCapture := engine.requests[0].BeforeCaptureJS
	if strings.Contains(beforeCapture, "scenario-nav-capture") {
		t.Fatalf("expected screen replace mode to omit scenario navigation hooks, got %s", beforeCapture)
	}
	if !strings.Contains(beforeCapture, "screen-nav-capture") {
		t.Fatalf("expected compiled screen before capture hook script, got %s", beforeCapture)
	}
}

func TestGenerateWorkflowReportWritesArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 240, G: 240, B: 240, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 230, G: 200, B: 200, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG current: %v", err)
	}
	captureMetadata := CaptureResult{
		OutputPath: currentPath,
		Warnings:   []CaptureWarning{{Code: "capture_warning", Message: "demo warning"}},
	}
	encoded, err := json.Marshal(captureMetadata)
	if err != nil {
		t.Fatalf("marshal capture metadata: %v", err)
	}
	if writeErr := os.WriteFile(currentPath+".json", append(encoded, '\n'), 0o644); writeErr != nil {
		t.Fatalf("write capture metadata: %v", writeErr)
	}

	scenario := WorkflowScenario{
		ID:          "report-test",
		Label:       "Report Test",
		Description: "workflow report fixture",
		SourceDir:   tempDir,
		SourcePath:  filepath.Join(tempDir, "report-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Priority: "P1", Title: "Patient review"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Screens: []WorkflowScreen{
			{
				ID:               "patient",
				Label:            "Patient",
				Route:            "/triage/patient",
				OutputName:       "patient",
				ReferenceImage:   referencePath,
				PrimaryStories:   []string{"D1"},
				ExpectedElements: []string{"#patient-review-proceed-btn"},
				Annotations:      []string{"Focus on patient summary hierarchy."},
				Evidence: []WorkflowEvidenceItem{
					{ID: "patient-direct-entry", Text: "Review step renders", Stories: []string{"D1"}},
				},
			},
		},
	}

	service := NewService(nil)
	service.now = func() time.Time {
		return time.Date(2026, time.March, 30, 20, 46, 0, 0, time.UTC)
	}
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("unexpected entry count: %d", len(result.Entries))
	}
	if result.Entries[0].CurrentCapture == nil {
		t.Fatal("expected report entry to include current capture metadata")
	}
	if result.Entries[0].DiffImagePath == "" {
		t.Fatal("expected diff image path")
	}
	if result.Entries[0].DiffEntry == nil {
		t.Fatal("expected diff entry metadata")
	}
	if len(result.Entries[0].Warnings) == 0 {
		t.Fatal("expected capture warnings to be carried into report entry")
	}
	if result.Entries[0].Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning entry status, got %+v", result.Entries[0].Status)
	}
	if result.Stories[0].Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning story status, got %+v", result.Stories[0].Status)
	}
	if result.Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning report status, got %+v", result.Status)
	}
	if _, statErr := os.Stat(result.ReportPath); statErr != nil {
		t.Fatalf("expected html report: %v", statErr)
	}
	if _, statErr := os.Stat(result.MetadataPath); statErr != nil {
		t.Fatalf("expected report metadata: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "styles.css")); statErr != nil {
		t.Fatalf("expected report stylesheet: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "current", "patient.png")); statErr != nil {
		t.Fatalf("expected staged current image asset: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "reference", filepath.Base(referencePath))); statErr != nil {
		t.Fatalf("expected staged reference image asset: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "diff", "patient.png")); statErr != nil {
		t.Fatalf("expected staged diff image asset: %v", statErr)
	}
	if _, statErr := os.Stat(result.Entries[0].DiffImagePath); statErr != nil {
		t.Fatalf("expected diff image: %v", statErr)
	}

	reportHTML, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report html: %v", err)
	}
	reportMarkup := string(reportHTML)
	reportStylesheet, err := os.ReadFile(filepath.Join(scenario.Artifacts.ReportDir, "styles.css"))
	if err != nil {
		t.Fatalf("read report stylesheet: %v", err)
	}
	reportCSS := string(reportStylesheet)
	for _, expected := range []string{
		"Print / Save PDF",
		"Generated Mar 30, 2026 at 8:46 PM",
		"Capture Issues",
		"Focus on patient summary hierarchy.",
		"data-screen-id=\"patient\"",
		"function setCompareMode(index, mode)",
		"el.dataset.screenId === screenID",
		"class=\"story-list-item-main\"",
		"data-story-compare-section",
		"style=\"--clip-right: 50%\"",
		"slider.style.setProperty('--clip-right', '50%')",
		"percent.toFixed(1) + '%'",
	} {
		if !strings.Contains(reportMarkup, expected) {
			t.Fatalf("expected report html to contain %q", expected)
		}
	}
	if strings.Contains(reportMarkup, "%%") {
		t.Fatal("expected rendered report html to avoid invalid %% percent literals")
	}
	for _, expected := range []string{
		".story-list-item-main { flex: 1 1 auto; min-width: 0; overflow: hidden; }",
		".story-list-item .pill { margin-left: auto; }",
		"clip-path: inset(0 var(--clip-right, 50%) 0 0);",
		"left: calc(100% - var(--clip-right, 50%));",
		"touch-action: none;",
	} {
		if !strings.Contains(reportCSS, expected) {
			t.Fatalf("expected report stylesheet to contain %q", expected)
		}
	}
	if strings.Contains(reportCSS, "%%") {
		t.Fatal("expected generated stylesheet to avoid invalid %% percent literals")
	}
	if strings.Contains(reportMarkup, "fonts.googleapis.com") {
		t.Fatal("expected report html to avoid remote font dependencies")
	}
	if !strings.Contains(reportMarkup, "document.querySelectorAll('.compare-mode-btn').forEach(btn => {") {
		t.Fatal("expected compare-mode event binding block in report html")
	}
	if !strings.Contains(reportMarkup, "document.addEventListener('keydown', (e) => {") {
		t.Fatal("expected keyboard navigation block in report html")
	}
}

func TestGenerateWorkflowReportUsesComparisonAssets(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 230, G: 230, B: 255, A: 255},
		{R: 210, G: 210, B: 255, A: 255},
		{R: 190, G: 190, B: 255, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 245, B: 245, A: 255},
		{R: 255, G: 235, B: 235, A: 255},
		{R: 255, G: 225, B: 225, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG current: %v", err)
	}

	scenario := WorkflowScenario{
		ID:         "report-compare-test",
		Label:      "Report Compare Test",
		SourceDir:  tempDir,
		SourcePath: filepath.Join(tempDir, "report-compare-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Title: "Patient review"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			Comparison: WorkflowComparison{
				Mode: WorkflowComparisonModeContentOnly,
				CurrentCrop: &WorkflowCompareRect{
					X: 0, Y: 0, Width: 4, Height: 1,
				},
				ReferenceCrop: &WorkflowCompareRect{
					X: 1, Y: 0, Width: 2, Height: 1,
				},
				ResizeTo: WorkflowComparisonResizeCurrent,
			},
		},
		Screens: []WorkflowScreen{
			{
				ID:             "patient",
				Label:          "Patient",
				Route:          "/triage/patient",
				OutputName:     "patient",
				ReferenceImage: referencePath,
				PrimaryStories: []string{"D1"},
			},
		},
	}

	service := NewService(nil)
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	entry := result.Entries[0]
	if entry.Status.Level != workflowStatusInfo {
		t.Fatalf("expected info entry status for visual diff, got %+v", entry.Status)
	}
	if result.Stories[0].Status.Level != workflowStatusInfo {
		t.Fatalf("expected info story status for visual diff, got %+v", result.Stories[0].Status)
	}
	if result.Status.Level != workflowStatusInfo {
		t.Fatalf("expected info report status for visual diff, got %+v", result.Status)
	}
	if entry.ComparedCurrentImagePath == "" || entry.ComparedReferenceImagePath == "" {
		t.Fatalf("expected compared image paths, got current=%q reference=%q", entry.ComparedCurrentImagePath, entry.ComparedReferenceImagePath)
	}
	if filepath.Base(entry.ComparedCurrentImagePath) != "patient-current.png" {
		t.Fatalf("unexpected compared current asset: %s", entry.ComparedCurrentImagePath)
	}
	if filepath.Base(entry.ComparedReferenceImagePath) != "patient-reference.png" {
		t.Fatalf("unexpected compared reference asset: %s", entry.ComparedReferenceImagePath)
	}

	currentCompared, err := loadImage(entry.ComparedCurrentImagePath)
	if err != nil {
		t.Fatalf("load compared current: %v", err)
	}
	referenceCompared, err := loadImage(entry.ComparedReferenceImagePath)
	if err != nil {
		t.Fatalf("load compared reference: %v", err)
	}
	if currentCompared.Bounds().Size() != image.Pt(4, 1) {
		t.Fatalf("unexpected compared current size: %v", currentCompared.Bounds().Size())
	}
	if referenceCompared.Bounds().Size() != image.Pt(4, 1) {
		t.Fatalf("unexpected compared reference size: %v", referenceCompared.Bounds().Size())
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "current", "patient-current.png")); err != nil {
		t.Fatalf("expected staged compared current image asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "reference", "patient-reference.png")); err != nil {
		t.Fatalf("expected staged compared reference image asset: %v", err)
	}
}

func TestGenerateWorkflowReportRunsSemanticDiffChangedOnly(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 200, G: 200, B: 200, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	provider := &fakeSemanticProvider{resp: SemanticProviderResponse{
		Provider: "openai",
		Model:    "gpt-test",
		RawText:  `{"summary":"Primary CTA moved","verdict":"needs_review","severity":"major","differences":[{"area":"CTA","description":"CTA moved lower","severity":"major","evidence":"button appears lower"}]}`,
	}}
	scenario := WorkflowScenario{
		ID:         "semantic-report-test",
		Label:      "Semantic Report Test",
		SourceDir:  tempDir,
		SourcePath: filepath.Join(tempDir, "semantic-report-test.yaml"),
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:  new(true),
				Provider: "openai",
				Model:    "gpt-test",
				Mode:     SemanticDiffModeFocused,
				Run:      SemanticDiffRunChangedOnly,
				Focus:    []string{"primary CTA"},
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Label:          "Patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			Providers: map[string]SemanticDiffProvider{"openai": provider},
		},
	})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	entry := result.Entries[0]
	if entry.SemanticDiff == nil {
		t.Fatal("expected semantic diff result")
	}
	if entry.SemanticDiff.Summary != "Primary CTA moved" || entry.SemanticMetadataPath == "" {
		t.Fatalf("unexpected semantic result: %#v", entry.SemanticDiff)
	}
	if len(provider.lastReq.Images) != 3 {
		t.Fatalf("expected semantic provider to receive current/reference/diff images, got %d", len(provider.lastReq.Images))
	}
	if _, err := os.Stat(entry.SemanticMetadataPath); err != nil {
		t.Fatalf("expected semantic metadata path: %v", err)
	}
	reportHTML, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report html: %v", err)
	}
	if !strings.Contains(string(reportHTML), "Semantic Diff") || !strings.Contains(string(reportHTML), "Primary CTA moved") || !strings.Contains(string(reportHTML), "CTA moved lower") {
		t.Fatalf("expected semantic findings in report html")
	}
}

func TestGenerateWorkflowReportUsesDefaultBuiltInSemanticProvider(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer workflow-key" {
			t.Fatalf("expected OpenAI auth header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"Workflow semantic ok\",\"verdict\":\"needs_review\",\"severity\":\"minor\"}"}]}]}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 200, G: 200, B: 200, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}

	scenario := WorkflowScenario{
		ID:        "semantic-default-provider-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:  new(true),
				Provider: "openai",
				Model:    "gpt-test",
				Run:      SemanticDiffRunAlways,
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "workflow-key", nil },
		OpenAIBaseURL:      server.URL,
	}})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one built-in provider call, got %d", calls)
	}
	if result.Entries[0].SemanticDiff == nil || result.Entries[0].SemanticDiff.Summary != "Workflow semantic ok" {
		t.Fatalf("unexpected semantic result: %#v", result.Entries[0].SemanticDiff)
	}
}

func TestGenerateWorkflowReportUsesCodexCLIProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 200, G: 200, B: 200, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	fake := writeWorkflowFakeCodex(t, tempDir, `#!/bin/sh
cat >/dev/null
printf '{"summary":"Workflow codex ok","verdict":"no_meaningful_change","severity":"info"}\n'
`)

	scenario := WorkflowScenario{
		ID:        "semantic-codex-provider-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:  new(true),
				Provider: "codex-cli",
				Model:    "gpt-test",
				Run:      SemanticDiffRunAlways,
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		LLMs: llms.Options{CodexCLI: llms.CodexCLIOptions{CommandPath: fake}},
	}})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if result.Entries[0].SemanticDiff == nil || result.Entries[0].SemanticDiff.Summary != "Workflow codex ok" {
		t.Fatalf("unexpected semantic result: %#v", result.Entries[0].SemanticDiff)
	}
}

func TestWorkflowSemanticDiffChangedOnlySkipsUnchanged(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	provider := &fakeSemanticProvider{resp: SemanticProviderResponse{
		Provider: "openai",
		RawText:  `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`,
	}}
	scenario := WorkflowScenario{
		ID:        "semantic-skip-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:  new(true),
				Provider: "openai",
				Run:      SemanticDiffRunChangedOnly,
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{Providers: map[string]SemanticDiffProvider{"openai": provider}}})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if result.Entries[0].SemanticDiff != nil {
		t.Fatalf("expected semantic diff to be skipped for unchanged pixel diff")
	}
	if provider.lastReq.Prompt != "" {
		t.Fatalf("expected provider not to be called")
	}
}

func TestWorkflowSemanticDiffProviderFailureBecomesWarning(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 200, G: 200, B: 200, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	scenario := WorkflowScenario{
		ID:        "semantic-warning-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:  new(true),
				Provider: "openai",
				Run:      SemanticDiffRunAlways,
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		Providers: map[string]SemanticDiffProvider{"openai": &fakeSemanticProvider{name: "openai", err: errors.New("semantic unavailable")}},
	}})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if result.Entries[0].SemanticDiff != nil {
		t.Fatal("expected no semantic result after provider failure")
	}
	if len(result.Entries[0].Warnings) == 0 || result.Entries[0].Status.Level != workflowStatusWarning {
		t.Fatalf("expected semantic provider failure warning, got entry %#v", result.Entries[0])
	}
}

func TestWorkflowSemanticDiffRawResponseRequiresProcessEnablement(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	rawPath := filepath.Join(tempDir, "semantic.raw.txt")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 200, G: 200, B: 200, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	scenario := WorkflowScenario{
		ID:        "semantic-raw-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled:            new(true),
				Provider:           "openai",
				Model:              "gpt-test",
				Run:                SemanticDiffRunAlways,
				PersistRawResponse: true,
				RawResponsePath:    rawPath,
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			OutputName:     "patient",
			ReferenceImage: referencePath,
		}},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		Providers: map[string]SemanticDiffProvider{"openai": &fakeSemanticProvider{name: "openai", resp: SemanticProviderResponse{
			Provider: "openai",
			Model:    "gpt-test",
			RawText:  `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`,
		}}},
	}})
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if result.Entries[0].SemanticDiff == nil {
		t.Fatal("expected semantic diff result")
	}
	if result.Entries[0].SemanticDiff.RawResponsePath != "" {
		t.Fatalf("expected raw response path to be omitted without process enablement: %#v", result.Entries[0].SemanticDiff)
	}
	if _, err := os.Stat(rawPath); !os.IsNotExist(err) {
		t.Fatalf("expected raw response file to be absent, stat err=%v", err)
	}
}

func TestWorkflowSemanticDiffRejectsCredentialFields(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	scenario := WorkflowScenario{
		ID:        "semantic-credential-test",
		SourceDir: tempDir,
		Environment: WorkflowEnvironment{
			BaseURL: "http://localhost:8383",
		},
		Defaults: WorkflowDefaults{
			SemanticDiff: WorkflowSemanticDiff{
				Enabled: new(true),
				APIKey:  "secret",
			},
		},
		Screens: []WorkflowScreen{{
			ID:             "patient",
			Route:          "/triage/patient",
			ReferenceImage: referencePath,
		}},
	}
	if err := normalizeWorkflowScenario(&scenario, WorkflowOptions{}); err == nil {
		t.Fatal("expected semantic diff credential validation error")
	}
}

func TestWorkflowSemanticDiffPolicyCanEscalate(t *testing.T) {
	result := SemanticDiffResult{Severity: SemanticDiffSeverityMajor, Verdict: SemanticDiffVerdictRegression}
	if workflowSemanticDiffFailsPolicy(WorkflowSemanticDiff{AdvisoryPolicy: SemanticDiffAdvisoryOnly, FailureSeverity: SemanticDiffSeverityMajor}, result) {
		t.Fatal("expected advisory policy to avoid semantic failure")
	}
	if !workflowSemanticDiffFailsPolicy(WorkflowSemanticDiff{AdvisoryPolicy: SemanticDiffAdvisoryEnforce, FailureSeverity: SemanticDiffSeverityMajor}, result) {
		t.Fatal("expected major severity to fail explicit policy")
	}
	if !workflowSemanticDiffFailsPolicy(WorkflowSemanticDiff{AdvisoryPolicy: SemanticDiffAdvisoryEnforce, FailureVerdicts: []SemanticDiffVerdict{SemanticDiffVerdictRegression}}, result) {
		t.Fatal("expected regression verdict to fail explicit policy")
	}
}

func TestGenerateWorkflowReportRendersMultiScreenStoryComparison(t *testing.T) {
	tempDir := t.TempDir()
	patientReferencePath := filepath.Join(tempDir, "patient-reference.png")
	recommendationReferencePath := filepath.Join(tempDir, "recommendation-reference.png")
	patientCurrentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	recommendationCurrentPath := filepath.Join(tempDir, "artifacts", "current", "recommendation.png")

	for _, fixture := range []struct {
		path   string
		pixels []color.NRGBA
	}{
		{
			path: patientReferencePath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 245, G: 245, B: 255, A: 255},
			},
		},
		{
			path: recommendationReferencePath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 255, G: 245, B: 245, A: 255},
			},
		},
		{
			path: patientCurrentPath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 235, G: 235, B: 255, A: 255},
			},
		},
		{
			path: recommendationCurrentPath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 255, G: 235, B: 235, A: 255},
			},
		},
	} {
		if err := writeTestPNG(fixture.path, fixture.pixels); err != nil {
			t.Fatalf("writeTestPNG %s: %v", filepath.Base(fixture.path), err)
		}
	}

	scenario := WorkflowScenario{
		ID:         "report-multi-story-test",
		Label:      "Report Multi Story Test",
		SourceDir:  tempDir,
		SourcePath: filepath.Join(tempDir, "report-multi-story-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Title: "Provider reviews multiple linked screens"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Screens: []WorkflowScreen{
			{
				ID:             "patient",
				Label:          "Patient",
				Route:          "/triage/patient",
				OutputName:     "patient",
				ReferenceImage: patientReferencePath,
				PrimaryStories: []string{"D1"},
			},
			{
				ID:             "recommendation",
				Label:          "Recommendation",
				Route:          "/triage/recommendation",
				OutputName:     "recommendation",
				ReferenceImage: recommendationReferencePath,
				PrimaryStories: []string{"D1"},
			},
		},
	}

	service := NewService(nil)
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if len(result.Stories) != 1 {
		t.Fatalf("unexpected story count: %d", len(result.Stories))
	}
	if got := result.Stories[0].ScreenIDs; len(got) != 2 || got[0] != "patient" || got[1] != "recommendation" {
		t.Fatalf("unexpected story screen ids: %#v", got)
	}

	reportHTML, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report html: %v", err)
	}
	reportMarkup := string(reportHTML)
	for _, expected := range []string{
		`data-story="D1"`,
		`data-screen-id="patient" data-thumb-index="0"`,
		`data-screen-id="recommendation" data-thumb-index="1"`,
		`const storedIndex = storyCompareIndex[storyID];`,
		`const initialIndex = Number.isInteger(storedIndex) && storedIndex >= 0 && storedIndex < thumbs.length`,
		`setStoryCompareScreen(storyID, initialIndex);`,
	} {
		if !strings.Contains(reportMarkup, expected) {
			t.Fatalf("expected report html to contain %q", expected)
		}
	}
	if strings.Count(reportMarkup, "data-thumb-index=") != 2 {
		t.Fatalf("expected exactly two story comparison thumbnails, got %d", strings.Count(reportMarkup, "data-thumb-index="))
	}
}

func writeWorkflowStoryFixture(t *testing.T, path string) {
	t.Helper()
	payload := `# Workflow Stories

| Code | Priority | User Story | Acceptance Criteria | Notes |
| ---- | -------- | ---------- | ------------------- | ----- |
| D1 | P1 | As a tester, I want to review a captured page. | Page renders<br>Capture is stable | Fixture story |

## Wireframe References
`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write story fixture: %v", err)
	}
}

func writeWorkflowFakeCodex(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}

func writeWorkflowScenarioFixture(t *testing.T, dir string) string {
	t.Helper()
	storyPath := filepath.Join(dir, "stories.md")
	referencePath := filepath.Join(dir, "reference.png")
	writeWorkflowStoryFixture(t, storyPath)
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference fixture: %v", err)
	}
	scenarioPath := filepath.Join(dir, "scenario.yaml")
	payload := `id: fixture-flow
label: Fixture Flow
story_source: stories.md
environment:
  base_url: http://localhost:8383
artifacts:
  root: artifacts
  reference_dir: .
defaults:
  viewport:
    width: 1024
    height: 1200
  full_page: true
stories: {}
screens:
  - id: patient
    label: Patient
    route: /triage/patient
    output_name: patient
    reference_image: reference.png
    primary_stories:
      - D1
`
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write scenario fixture: %v", err)
	}
	return scenarioPath
}
