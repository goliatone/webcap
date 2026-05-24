package webcap

import (
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSemanticProvider struct {
	name      string
	resp      SemanticProviderResponse
	err       error
	lastReq   SemanticProviderRequest
	onCompare func(SemanticProviderRequest)
}

func (p *fakeSemanticProvider) Name() string {
	if p.name == "" {
		return "fake"
	}
	return p.name
}

func (p *fakeSemanticProvider) CompareImages(_ context.Context, req SemanticProviderRequest) (SemanticProviderResponse, error) {
	p.lastReq = req
	if p.onCompare != nil {
		p.onCompare(req)
	}
	if p.err != nil {
		return SemanticProviderResponse{}, p.err
	}
	return p.resp, nil
}

func TestNormalizeSemanticDiffRequestRejectsInvalidValues(t *testing.T) {
	cases := []SemanticDiffRequest{
		{ReferencePath: "ref.png"},
		{CurrentPath: "cur.png"},
		{CurrentPath: "cur.png", ReferencePath: "ref.png", Mode: "unknown"},
		{CurrentPath: "cur.png", ReferencePath: "ref.png", Mode: SemanticDiffModeCustom},
		{CurrentPath: "cur.png", ReferencePath: "ref.png", Timeout: "soon"},
		{CurrentPath: "cur.png", ReferencePath: "ref.png", MaxOutputTokens: -1},
		{CurrentPath: "cur.png", ReferencePath: "ref.png", RawResponsePath: "raw.txt"},
	}
	for _, req := range cases {
		if _, err := NormalizeSemanticDiffRequest(req); err == nil {
			t.Fatalf("expected validation error for %#v", req)
		}
	}
}

func TestServiceSemanticDiffUsesFakeProviderAndMetadata(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	diffPath := filepath.Join(dir, "diff.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 0, G: 0, B: 0, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(diffPath, []color.NRGBA{{R: 255, G: 0, B: 102, A: 255}}); err != nil {
		t.Fatalf("write diff: %v", err)
	}

	provider := &fakeSemanticProvider{resp: SemanticProviderResponse{
		Provider: "fake",
		Model:    "vision-test",
		RawText:  `{"summary":"CTA moved","verdict":"needs_review","severity":"major","differences":[{"area":"hero","description":"CTA is lower","severity":"major"}]}`,
	}}
	service := NewServiceWithOptions(nil, Options{
		Now: func() time.Time { return time.Unix(100, 0).UTC() },
		SemanticDiff: SemanticDiffOptions{
			DefaultProvider: "fake",
			DefaultModels:   map[string]string{"fake": "vision-test"},
			Providers:       map[string]SemanticDiffProvider{"fake": provider},
		},
	})
	metadataPath := filepath.Join(dir, "semantic.json")
	result, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:   currentPath,
		ReferencePath: referencePath,
		Mode:          SemanticDiffModeFocused,
		Focus:         []string{"primary CTA"},
		PixelContext: SemanticPixelContext{
			PixelDiffImagePath: diffPath,
			ChangedPixels:      1,
			TotalPixels:        1,
			ChangedPercent:     100,
		},
		MetadataPath: metadataPath,
	})
	if err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if result.Summary != "CTA moved" || result.Verdict != SemanticDiffVerdictNeedsReview || result.Severity != SemanticDiffSeverityMajor {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(provider.lastReq.Images) != 3 {
		t.Fatalf("expected current/reference/pixel images, got %d", len(provider.lastReq.Images))
	}
	if strings.Contains(provider.lastReq.Prompt, "OPENAI_API_KEY") {
		t.Fatal("prompt leaked credential material")
	}
	payload, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if strings.Contains(string(payload), provider.resp.RawText) {
		t.Fatal("metadata should not persist raw provider payload by default")
	}
	var persisted SemanticDiffResult
	if err := json.Unmarshal(payload, &persisted); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if persisted.Prompt.Mode != SemanticDiffModeFocused || !persisted.Prompt.PixelContextIncluded {
		t.Fatalf("unexpected prompt metadata: %#v", persisted.Prompt)
	}
}

func TestServiceSemanticDiffProviderErrors(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	service := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			DefaultProvider: "fake",
			Providers: map[string]SemanticDiffProvider{"fake": &fakeSemanticProvider{
				err: errors.New("provider failed"),
			}},
		},
	})
	if _, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{CurrentPath: currentPath, ReferencePath: referencePath}); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestServiceSemanticDiffPersistsRawResponseOnlyWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	raw := `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`
	service := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			DefaultProvider: "fake",
			Providers: map[string]SemanticDiffProvider{"fake": &fakeSemanticProvider{resp: SemanticProviderResponse{
				Provider: "fake",
				RawText:  raw,
			}}},
		},
	})
	metadataPath := filepath.Join(dir, "semantic.json")
	rawPath := filepath.Join(dir, "raw.txt")
	result, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:        currentPath,
		ReferencePath:      referencePath,
		MetadataPath:       metadataPath,
		RawResponsePath:    rawPath,
		PersistRawResponse: true,
	})
	if err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if result.RawResponse != raw || result.RawResponsePath != rawPath {
		t.Fatalf("expected in-memory raw response and path, got %#v", result)
	}
	rawPayload, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw response: %v", err)
	}
	if !strings.Contains(string(rawPayload), raw) {
		t.Fatalf("expected raw response file to contain provider text")
	}
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if strings.Contains(string(metadata), raw) {
		t.Fatal("metadata should exclude raw provider payload even when raw persistence is enabled")
	}
}

func TestServiceSemanticDiffBudgetsAndResizesProviderCopies(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	pixels := make([]color.NRGBA, 120)
	for i := range pixels {
		pixels[i] = color.NRGBA{R: uint8(i), G: 50, B: 10, A: 255}
	}
	if err := writeTestPNG(currentPath, pixels); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}

	var providerPath string
	provider := &fakeSemanticProvider{
		resp: SemanticProviderResponse{Provider: "fake", RawText: `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`},
		onCompare: func(req SemanticProviderRequest) {
			providerPath = req.Images[0].Path
			if providerPath == currentPath {
				t.Fatalf("expected resized provider copy, got original path")
			}
			if _, err := os.Stat(providerPath); err != nil {
				t.Fatalf("provider copy did not exist during provider call: %v", err)
			}
		},
	}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		DefaultProvider:  "fake",
		Providers:        map[string]SemanticDiffProvider{"fake": provider},
		MaxImageLongEdge: 20,
		ResizeImages:     true,
	}})
	result, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:   currentPath,
		ReferencePath: referencePath,
		MetadataPath:  filepath.Join(dir, "semantic.json"),
	})
	if err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if len(result.ImageMetadata) == 0 || result.ImageMetadata[0].ResizeReason != "max_long_edge" || result.ImageMetadata[0].ProviderPath == "" {
		t.Fatalf("expected resize metadata, got %#v", result.ImageMetadata)
	}
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("original image was not preserved: %v", err)
	}
	if _, err := os.Stat(providerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected provider copy cleanup after call, stat err=%v", err)
	}
}

func TestServiceSemanticDiffRejectsCombinedEncodedBudget(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	provider := &fakeSemanticProvider{resp: SemanticProviderResponse{RawText: `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`}}
	service := NewServiceWithOptions(nil, Options{SemanticDiff: SemanticDiffOptions{
		DefaultProvider:              "fake",
		Providers:                    map[string]SemanticDiffProvider{"fake": provider},
		MaxCombinedEncodedImageBytes: 1,
	}})
	_, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{CurrentPath: currentPath, ReferencePath: referencePath})
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Code != CodeProviderPayloadTooLarge || provider.lastReq.Provider != "" {
		t.Fatalf("expected encoded budget failure before provider call, got err=%v capture=%#v req=%#v", err, captureErr, provider.lastReq)
	}
}

func TestServiceSemanticDiffRedactionHookCanReplaceOrAbortImages(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	redactedPath := filepath.Join(dir, "redacted.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{R: 255, A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{G: 255, A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	if err := writeTestPNG(redactedPath, []color.NRGBA{{B: 255, A: 255}}); err != nil {
		t.Fatalf("write redacted: %v", err)
	}
	provider := &fakeSemanticProvider{resp: SemanticProviderResponse{
		Provider: "fake",
		RawText:  `{"summary":"ok","verdict":"no_meaningful_change","severity":"info"}`,
	}}
	service := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			DefaultProvider: "fake",
			Providers:       map[string]SemanticDiffProvider{"fake": provider},
			RedactImage: func(_ context.Context, input SemanticImageRedactionInput) (string, error) {
				if input.Role == "current" {
					return redactedPath, nil
				}
				return input.Path, nil
			},
		},
	})
	if _, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{
		CurrentPath:   currentPath,
		ReferencePath: referencePath,
		MetadataPath:  filepath.Join(dir, "redacted.semantic.json"),
	}); err != nil {
		t.Fatalf("SemanticDiff returned error: %v", err)
	}
	if provider.lastReq.Images[0].Path != redactedPath {
		t.Fatalf("expected current image to be replaced by redaction hook, got %#v", provider.lastReq.Images[0])
	}

	abortService := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			DefaultProvider: "fake",
			Providers:       map[string]SemanticDiffProvider{"fake": provider},
			RedactImage: func(context.Context, SemanticImageRedactionInput) (string, error) {
				return "", errors.New("redaction failed")
			},
		},
	})
	if _, err := abortService.SemanticDiff(context.Background(), SemanticDiffRequest{CurrentPath: currentPath, ReferencePath: referencePath}); err == nil {
		t.Fatal("expected redaction hook error")
	}
}

func TestSemanticDiffRequiresConfiguredProvider(t *testing.T) {
	service := NewService(nil)
	if _, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{CurrentPath: "current.png", ReferencePath: "reference.png", Provider: "unknown"}); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestSemanticDiffBuiltInProviderRequiresModel(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	if err := writeTestPNG(currentPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := writeTestPNG(referencePath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	service := NewServiceWithOptions(nil, Options{
		SemanticDiff: SemanticDiffOptions{
			Providers: map[string]SemanticDiffProvider{"openai": &fakeSemanticProvider{name: "openai"}},
		},
	})
	if _, err := service.SemanticDiff(context.Background(), SemanticDiffRequest{CurrentPath: currentPath, ReferencePath: referencePath, Provider: "openai"}); err == nil {
		t.Fatal("expected missing built-in provider model error")
	}
}
