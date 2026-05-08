package webcap

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) Diff(ctx context.Context, req DiffRequest) (DiffResult, error) {
	normalized, err := NormalizeDiffRequest(req)
	if err != nil {
		return DiffResult{}, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return DiffResult{}, wrapCaptureError("diff", ctxErr)
	}

	mode, err := InferDiffMode(normalized.BasePath, normalized.ComparePath)
	if err != nil {
		return DiffResult{}, err
	}
	resolved, err := ResolveDiffPaths(normalized, mode)
	if err != nil {
		return DiffResult{}, err
	}

	switch mode {
	case DiffModeDirectory:
		return s.diffDirectories(ctx, resolved)
	default:
		return s.diffSingle(ctx, resolved)
	}
}

func (s *Service) diffSingle(ctx context.Context, req DiffRequest) (DiffResult, error) {
	if err := ctx.Err(); err != nil {
		return DiffResult{}, wrapCaptureError("diff_single", err)
	}
	entry, payload, err := diffImagePair(req.BasePath, req.ComparePath, req.Threshold)
	if err != nil {
		return DiffResult{}, err
	}
	entry.OutputPath = req.OutputPath
	entry.MetadataPath = req.MetadataPath
	entry.ByteSize = len(payload)
	if err := writeFile(req.OutputPath, payload); err != nil {
		return DiffResult{}, wrapCaptureError("write_diff_image", err)
	}

	result := DiffResult{
		Mode:         DiffModeImage,
		BasePath:     req.BasePath,
		ComparePath:  req.ComparePath,
		OutputPath:   req.OutputPath,
		MetadataPath: req.MetadataPath,
		Threshold:    req.Threshold,
		Entry:        &entry,
		Summary: DiffSummary{
			ComparedFiles:      1,
			ChangedFiles:       boolToInt(entry.Changed),
			TotalChangedPixels: entry.ChangedPixels,
		},
		CreatedAt: s.now(),
	}
	if err := writeDiffMetadata(result.MetadataPath, result); err != nil {
		return DiffResult{}, err
	}
	return result, nil
}

func (s *Service) diffDirectories(ctx context.Context, req DiffRequest) (DiffResult, error) {
	baseFiles, err := listDiffableFiles(req.BasePath)
	if err != nil {
		return DiffResult{}, err
	}
	compareFiles, err := listDiffableFiles(req.ComparePath)
	if err != nil {
		return DiffResult{}, err
	}

	keys := diffFileKeys(baseFiles, compareFiles)
	entries := make([]DiffEntry, 0, len(keys))
	summary := DiffSummary{}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return DiffResult{}, wrapCaptureError("diff_directories", err)
		}
		basePath, hasBase := baseFiles[key]
		comparePath, hasCompare := compareFiles[key]
		entry, delta, err := diffDirectoryEntry(req, key, basePath, comparePath, hasBase, hasCompare)
		if err != nil {
			return DiffResult{}, err
		}
		mergeDiffSummary(&summary, delta)
		entries = append(entries, entry)
	}

	result := DiffResult{
		Mode:         DiffModeDirectory,
		BasePath:     req.BasePath,
		ComparePath:  req.ComparePath,
		OutputPath:   req.OutputPath,
		MetadataPath: req.MetadataPath,
		Threshold:    req.Threshold,
		Entries:      entries,
		Summary:      summary,
		CreatedAt:    s.now(),
	}
	if err := writeDiffMetadata(result.MetadataPath, result); err != nil {
		return DiffResult{}, err
	}
	return result, nil
}

func diffFileKeys(baseFiles, compareFiles map[string]string) []string {
	keys := make([]string, 0, len(baseFiles)+len(compareFiles))
	seen := map[string]struct{}{}
	for key := range baseFiles {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range compareFiles {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func diffDirectoryEntry(req DiffRequest, key, basePath, comparePath string, hasBase, hasCompare bool) (DiffEntry, DiffSummary, error) {
	entry := DiffEntry{
		RelativePath:   key,
		BasePath:       basePath,
		ComparePath:    comparePath,
		Threshold:      req.Threshold,
		MissingBase:    !hasBase,
		MissingCompare: !hasCompare,
	}
	switch {
	case !hasBase:
		entry.Changed = true
		entry.Warnings = append(entry.Warnings, CaptureWarning{Code: string(CodeValidation), Message: "missing file in base directory"})
		return entry, DiffSummary{MissingBaseFiles: 1, ChangedFiles: 1, Warnings: entry.Warnings}, nil
	case !hasCompare:
		entry.Changed = true
		entry.Warnings = append(entry.Warnings, CaptureWarning{Code: string(CodeValidation), Message: "missing file in compare directory"})
		return entry, DiffSummary{MissingCompareFiles: 1, ChangedFiles: 1, Warnings: entry.Warnings}, nil
	default:
		return comparedDirectoryEntry(req, key, basePath, comparePath)
	}
}

func comparedDirectoryEntry(req DiffRequest, key, basePath, comparePath string) (DiffEntry, DiffSummary, error) {
	entry, payload, err := diffImagePair(basePath, comparePath, req.Threshold)
	if err != nil {
		return DiffEntry{}, DiffSummary{}, err
	}
	entry.RelativePath = key
	entry.OutputPath = filepath.Join(req.OutputPath, normalizeDiffOutputRelative(key))
	entry.MetadataPath = entry.OutputPath + ".json"
	entry.ByteSize = len(payload)
	if err := writeFile(entry.OutputPath, payload); err != nil {
		return DiffEntry{}, DiffSummary{}, wrapCaptureError("write_diff_image", err)
	}
	if err := writeDiffMetadata(entry.MetadataPath, entry); err != nil {
		return DiffEntry{}, DiffSummary{}, err
	}

	summary := DiffSummary{
		ComparedFiles:      1,
		ChangedFiles:       boolToInt(entry.Changed),
		TotalChangedPixels: entry.ChangedPixels,
	}
	return entry, summary, nil
}

func mergeDiffSummary(summary *DiffSummary, delta DiffSummary) {
	summary.ComparedFiles += delta.ComparedFiles
	summary.ChangedFiles += delta.ChangedFiles
	summary.MissingBaseFiles += delta.MissingBaseFiles
	summary.MissingCompareFiles += delta.MissingCompareFiles
	summary.TotalChangedPixels += delta.TotalChangedPixels
	summary.Warnings = append(summary.Warnings, delta.Warnings...)
}

func diffImagePair(basePath, comparePath string, threshold float64) (DiffEntry, []byte, error) {
	baseImage, err := loadImage(basePath)
	if err != nil {
		return DiffEntry{}, nil, err
	}
	compareImage, err := loadImage(comparePath)
	if err != nil {
		return DiffEntry{}, nil, err
	}

	baseNRGBA := toNRGBA(baseImage)
	compareNRGBA := toNRGBA(compareImage)
	bounds := unionBounds(baseNRGBA.Bounds(), compareNRGBA.Bounds())
	diffCanvas := image.NewNRGBA(bounds)

	totalPixels := bounds.Dx() * bounds.Dy()
	changedPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			basePixel, baseInside := pixelAt(baseNRGBA, x, y)
			comparePixel, compareInside := pixelAt(compareNRGBA, x, y)
			changed, accent := diffPixel(basePixel, comparePixel, baseInside, compareInside, threshold)
			if changed {
				changedPixels++
				diffCanvas.Set(x, y, accent)
				continue
			}
			diffCanvas.Set(x, y, mutedPixel(comparePixel))
		}
	}

	payload, err := encodePNG(diffCanvas)
	if err != nil {
		return DiffEntry{}, nil, wrapCaptureError("encode_diff_image", err)
	}

	entry := DiffEntry{
		BasePath:       basePath,
		ComparePath:    comparePath,
		Width:          bounds.Dx(),
		Height:         bounds.Dy(),
		TotalPixels:    totalPixels,
		ChangedPixels:  changedPixels,
		ChangedPercent: percent(changedPixels, totalPixels),
		Threshold:      threshold,
		Changed:        changedPixels > 0,
	}
	return entry, payload, nil
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, wrapCaptureError("open_diff_image", err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		_ = file.Close()
		return nil, wrapCaptureError("decode_diff_image", err)
	}
	if err := file.Close(); err != nil {
		return nil, wrapCaptureError("close_diff_image", err)
	}
	return img, nil
}

func listDiffableFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isDiffableExtension(path) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = path
		return nil
	})
	if err != nil {
		return nil, wrapCaptureError("walk_diff_directory", err)
	}
	return files, nil
}

func isDiffableExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}

func normalizeDiffOutputRelative(relative string) string {
	ext := filepath.Ext(relative)
	base := strings.TrimSuffix(relative, ext)
	return filepath.Clean(base + ".png")
}

func unionBounds(a, b image.Rectangle) image.Rectangle {
	width := max(a.Dx(), b.Dx())
	height := max(a.Dy(), b.Dy())
	return image.Rect(0, 0, width, height)
}

func toNRGBA(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), img, bounds.Min, draw.Src)
	return dst
}

func pixelAt(img *image.NRGBA, x, y int) (color.NRGBA, bool) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return color.NRGBA{}, false
	}
	offset := img.PixOffset(x, y)
	return color.NRGBA{
		R: img.Pix[offset],
		G: img.Pix[offset+1],
		B: img.Pix[offset+2],
		A: img.Pix[offset+3],
	}, true
}

func diffPixel(base, compare color.NRGBA, baseInside, compareInside bool, threshold float64) (bool, color.NRGBA) {
	if !baseInside || !compareInside {
		return true, color.NRGBA{R: 255, G: 140, B: 0, A: 255}
	}
	maxDelta := maxChannelDelta(base, compare)
	if maxDelta <= threshold {
		return false, color.NRGBA{}
	}
	return true, color.NRGBA{R: 255, G: 0, B: 102, A: 255}
}

func maxChannelDelta(a, b color.NRGBA) float64 {
	return max(
		absChannelDelta(a.R, b.R),
		max(
			absChannelDelta(a.G, b.G),
			max(absChannelDelta(a.B, b.B), absChannelDelta(a.A, b.A)),
		),
	)
}

func absChannelDelta(a, b uint8) float64 {
	if a > b {
		return float64(a-b) / 255
	}
	return float64(b-a) / 255
}

func mutedPixel(value color.NRGBA) color.NRGBA {
	gray := uint8((uint16(value.R) + uint16(value.G) + uint16(value.B)) / 3)
	alpha := value.A
	if alpha == 0 {
		alpha = 255
	}
	return color.NRGBA{R: gray, G: gray, B: gray, A: alpha}
}

func encodePNG(img image.Image) ([]byte, error) {
	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func percent(changed, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(changed) / float64(total) * 100
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func writeDiffMetadata(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return wrapCaptureError("marshal_diff_metadata", err)
	}
	if err := writeFile(path, append(encoded, '\n')); err != nil {
		return wrapCaptureError("write_diff_metadata", err)
	}
	return nil
}

func max[T ~int | ~float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
