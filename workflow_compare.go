package webcap

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"strings"
)

const (
	WorkflowComparisonModeFull        = "full"
	WorkflowComparisonModeContentOnly = "content_only"
	WorkflowComparisonResizeCurrent   = "current"
	WorkflowComparisonResizeReference = "reference"
)

func normalizeWorkflowComparison(value WorkflowComparison) (WorkflowComparison, error) {
	value.Mode = strings.TrimSpace(strings.ToLower(value.Mode))
	switch value.Mode {
	case "", WorkflowComparisonModeFull:
		value.Mode = WorkflowComparisonModeFull
	case WorkflowComparisonModeContentOnly:
	default:
		return WorkflowComparison{}, newCaptureError(CodeValidation, "normalize_workflow_comparison", fmt.Sprintf("unsupported workflow comparison mode %q", value.Mode), nil)
	}
	value.ResizeTo = strings.TrimSpace(strings.ToLower(value.ResizeTo))
	switch value.ResizeTo {
	case "", WorkflowComparisonResizeCurrent, WorkflowComparisonResizeReference:
	default:
		return WorkflowComparison{}, newCaptureError(CodeValidation, "normalize_workflow_comparison", fmt.Sprintf("unsupported workflow comparison resize_to %q", value.ResizeTo), nil)
	}
	if value.CurrentCrop != nil {
		crop, err := normalizeWorkflowCompareRect(*value.CurrentCrop)
		if err != nil {
			return WorkflowComparison{}, err
		}
		value.CurrentCrop = &crop
	}
	if value.ReferenceCrop != nil {
		crop, err := normalizeWorkflowCompareRect(*value.ReferenceCrop)
		if err != nil {
			return WorkflowComparison{}, err
		}
		value.ReferenceCrop = &crop
	}
	return value, nil
}

func normalizeWorkflowCompareRect(value WorkflowCompareRect) (WorkflowCompareRect, error) {
	if value.X < 0 || value.Y < 0 {
		return WorkflowCompareRect{}, newCaptureError(CodeValidation, "normalize_workflow_compare_rect", "comparison crop coordinates must be >= 0", nil)
	}
	if value.Width <= 0 || value.Height <= 0 {
		return WorkflowCompareRect{}, newCaptureError(CodeValidation, "normalize_workflow_compare_rect", "comparison crop width and height must be > 0", nil)
	}
	return value, nil
}

func mergeWorkflowComparison(base, override WorkflowComparison) WorkflowComparison {
	out := base
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	if override.CurrentCrop != nil {
		crop := *override.CurrentCrop
		out.CurrentCrop = &crop
	}
	if override.ReferenceCrop != nil {
		crop := *override.ReferenceCrop
		out.ReferenceCrop = &crop
	}
	if override.ResizeTo != "" {
		out.ResizeTo = override.ResizeTo
	}
	return out
}

func prepareWorkflowComparisonImages(entry WorkflowReportEntry, comparison WorkflowComparison, diffDir string) (string, string, error) {
	comparison, err := normalizeWorkflowComparison(comparison)
	if err != nil {
		return "", "", err
	}
	if comparison.Mode == WorkflowComparisonModeFull {
		return entry.CurrentImagePath, entry.ReferenceImage, nil
	}

	currentTarget := filepath.Join(diffDir, "compare", sanitizeName(entry.ScreenID)+"-current.png")
	referenceTarget := filepath.Join(diffDir, "compare", sanitizeName(entry.ScreenID)+"-reference.png")

	currentTarget, currentBounds, err := transformWorkflowComparisonImage(entry.CurrentImagePath, currentTarget, comparison.CurrentCrop, image.Point{})
	if err != nil {
		return "", "", err
	}

	targetSize := image.Point{}
	switch comparison.ResizeTo {
	case WorkflowComparisonResizeCurrent:
		targetSize = currentBounds
	}
	referenceTarget, referenceBounds, err := transformWorkflowComparisonImage(entry.ReferenceImage, referenceTarget, comparison.ReferenceCrop, targetSize)
	if err != nil {
		return "", "", err
	}

	if comparison.ResizeTo == WorkflowComparisonResizeReference && (referenceBounds.X > 0 && referenceBounds.Y > 0) {
		currentTarget, _, err = transformWorkflowComparisonImage(entry.CurrentImagePath, currentTarget, comparison.CurrentCrop, referenceBounds)
		if err != nil {
			return "", "", err
		}
	}

	return currentTarget, referenceTarget, nil
}

func transformWorkflowComparisonImage(sourcePath, outputPath string, crop *WorkflowCompareRect, resizeTo image.Point) (string, image.Point, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", image.Point{}, nil
	}
	source, err := loadImage(sourcePath)
	if err != nil {
		return "", image.Point{}, err
	}
	canvas := toNRGBA(source)
	if crop != nil {
		canvas, err = cropWorkflowImage(canvas, *crop)
		if err != nil {
			return "", image.Point{}, err
		}
	}
	if resizeTo.X > 0 && resizeTo.Y > 0 && (canvas.Bounds().Dx() != resizeTo.X || canvas.Bounds().Dy() != resizeTo.Y) {
		canvas = resizeWorkflowImage(canvas, resizeTo.X, resizeTo.Y)
	}
	payload, err := encodePNG(canvas)
	if err != nil {
		return "", image.Point{}, wrapCaptureError("encode_workflow_comparison_image", err)
	}
	if err := writeFile(outputPath, payload); err != nil {
		return "", image.Point{}, err
	}
	return outputPath, image.Pt(canvas.Bounds().Dx(), canvas.Bounds().Dy()), nil
}

func cropWorkflowImage(source *image.NRGBA, crop WorkflowCompareRect) (*image.NRGBA, error) {
	bounds := source.Bounds()
	clip := image.Rect(crop.X, crop.Y, crop.X+crop.Width, crop.Y+crop.Height).Intersect(bounds)
	if clip.Dx() <= 0 || clip.Dy() <= 0 {
		return nil, newCaptureError(CodeValidation, "crop_workflow_image", "comparison crop is outside the source image bounds", nil)
	}
	target := image.NewNRGBA(image.Rect(0, 0, clip.Dx(), clip.Dy()))
	draw.Draw(target, target.Bounds(), source, clip.Min, draw.Src)
	return target, nil
}

func resizeWorkflowImage(source *image.NRGBA, width, height int) *image.NRGBA {
	target := image.NewNRGBA(image.Rect(0, 0, width, height))
	srcBounds := source.Bounds()
	if srcBounds.Dx() == 0 || srcBounds.Dy() == 0 {
		return target
	}

	draw.Draw(target, target.Bounds(), &image.Uniform{C: color.NRGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	scaleX := float64(width) / float64(srcBounds.Dx())
	scaleY := float64(height) / float64(srcBounds.Dy())
	scale := math.Min(scaleX, scaleY)
	scaledWidth := int(math.Max(1, math.Round(float64(srcBounds.Dx())*scale)))
	scaledHeight := int(math.Max(1, math.Round(float64(srcBounds.Dy())*scale)))
	offsetX := (width - scaledWidth) / 2
	offsetY := 0

	for y := range scaledHeight {
		srcY := srcBounds.Min.Y + (y*srcBounds.Dy())/scaledHeight
		for x := range scaledWidth {
			srcX := srcBounds.Min.X + (x*srcBounds.Dx())/scaledWidth
			target.SetNRGBA(offsetX+x, offsetY+y, source.NRGBAAt(srcX, srcY))
		}
	}
	return target
}
