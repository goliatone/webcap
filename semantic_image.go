package webcap

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SemanticImagePayload struct {
	Role       string                `json:"role"`
	Path       string                `json:"path"`
	MIMEType   string                `json:"mime_type"`
	Base64Data string                `json:"-"`
	ByteSize   int64                 `json:"byte_size"`
	Width      int                   `json:"width,omitempty"`
	Height     int                   `json:"height,omitempty"`
	Metadata   SemanticImageMetadata `json:"-"`
}

type semanticImagePrepareOptions struct {
	MaxRawBytes      int64
	MaxProviderBytes int64
	MaxLongEdge      int
	MaxPixels        int64
	MaxEncodedBytes  int64
	ResizeImages     bool
	TempDir          string
	ScaleFactor      float64
}

func prepareSemanticImagePayload(path, role string, options semanticImagePrepareOptions) (SemanticImagePayload, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "image path is required", nil)
	}
	if options.MaxRawBytes <= 0 {
		options.MaxRawBytes = defaultSemanticDiffMaxImageBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("stat_semantic_image", err)
	}
	if info.IsDir() {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff image path must be a file", nil)
	}
	if info.Size() > options.MaxRawBytes {
		return SemanticImagePayload{}, semanticImageBudgetError("max_raw_bytes", SemanticImageMetadata{
			Role:             strings.TrimSpace(role),
			OriginalPath:     path,
			OriginalByteSize: info.Size(),
		}, options)
	}
	mimeType, err := semanticImageMIMEType(path)
	if err != nil {
		return SemanticImagePayload{}, err
	}
	payload, err := readSemanticImageFile(path)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("read_semantic_image", err)
	}
	if detected := http.DetectContentType(payload); strings.HasPrefix(detected, "image/") && detected != "application/octet-stream" {
		if detected == "image/png" || detected == "image/jpeg" {
			mimeType = detected
		}
	}
	width, height, err := semanticImageDimensions(payload)
	if err != nil {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff image could not be decoded", err)
	}
	metadata := SemanticImageMetadata{
		Role:             strings.TrimSpace(role),
		OriginalPath:     path,
		MIMEType:         mimeType,
		OriginalByteSize: int64(len(payload)),
		OriginalWidth:    width,
		OriginalHeight:   height,
		ProviderByteSize: int64(len(payload)),
		ProviderWidth:    width,
		ProviderHeight:   height,
		Limits:           semanticImageLimits(options),
	}
	resizeReason := semanticResizeReason(metadata, options)
	if resizeReason != "" {
		path, payload, metadata, err = prepareSemanticImageProviderCopy(path, role, mimeType, payload, metadata, options, resizeReason)
		if err != nil {
			return SemanticImagePayload{}, err
		}
		width = metadata.ProviderWidth
		height = metadata.ProviderHeight
	}
	return SemanticImagePayload{
		Role:       strings.TrimSpace(role),
		Path:       path,
		MIMEType:   mimeType,
		Base64Data: base64.StdEncoding.EncodeToString(payload),
		ByteSize:   int64(len(payload)),
		Width:      width,
		Height:     height,
		Metadata:   metadata,
	}, nil
}

func prepareSemanticImageProviderCopy(path, role, mimeType string, payload []byte, metadata SemanticImageMetadata, options semanticImagePrepareOptions, resizeReason string) (string, []byte, SemanticImageMetadata, error) {
	if !options.ResizeImages {
		return "", nil, SemanticImageMetadata{}, semanticImageBudgetError(resizeReason, metadata, options)
	}
	resized, err := resizeSemanticImage(payload, mimeType, options)
	if err != nil {
		return "", nil, SemanticImageMetadata{}, wrapCaptureError("resize_semantic_image", err)
	}
	width, height, _ := semanticImageDimensions(resized)
	metadata.ProviderByteSize = int64(len(resized))
	metadata.ProviderWidth = width
	metadata.ProviderHeight = height
	metadata.ResizeReason = resizeReason
	if still := semanticResizeReason(metadata, options); still != "" {
		return "", nil, SemanticImageMetadata{}, semanticImageBudgetError(still, metadata, options)
	}
	if options.TempDir == "" {
		return "", nil, SemanticImageMetadata{}, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic image resize requires a temporary directory", nil)
	}
	copyPath := filepath.Join(options.TempDir, semanticProviderCopyName(role, path, mimeType))
	if err := os.WriteFile(copyPath, resized, 0o600); err != nil {
		return "", nil, SemanticImageMetadata{}, wrapCaptureError("write_semantic_provider_image", err)
	}
	metadata.ProviderPath = copyPath
	return copyPath, resized, metadata, nil
}

func readSemanticImageFile(path string) ([]byte, error) {
	root, name, err := openPathRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	return io.ReadAll(file)
}

func openPathRoot(path string) (*os.Root, string, error) {
	path = filepath.Clean(path)
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, "", err
	}
	return root, filepath.Base(path), nil
}

func semanticImageMIMEType(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", nil
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	default:
		return "", newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff supports PNG, JPG, and JPEG images", nil)
	}
}

func semanticImageDimensions(payload []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func semanticResizeReason(metadata SemanticImageMetadata, options semanticImagePrepareOptions) string {
	if options.MaxProviderBytes > 0 && metadata.ProviderByteSize > options.MaxProviderBytes {
		return "max_provider_bytes"
	}
	longEdge := metadata.ProviderWidth
	if metadata.ProviderHeight > longEdge {
		longEdge = metadata.ProviderHeight
	}
	if options.MaxLongEdge > 0 && longEdge > options.MaxLongEdge {
		return "max_long_edge"
	}
	if options.MaxPixels > 0 && int64(metadata.ProviderWidth)*int64(metadata.ProviderHeight) > options.MaxPixels {
		return "max_pixels"
	}
	if options.MaxEncodedBytes > 0 && int64(base64.StdEncoding.EncodedLen(int(metadata.ProviderByteSize))) > options.MaxEncodedBytes {
		return "max_encoded_image_bytes"
	}
	return ""
}

func semanticImageBudgetError(limitName string, metadata SemanticImageMetadata, options semanticImagePrepareOptions) error {
	actual := metadata.ProviderByteSize
	limit := options.MaxRawBytes
	switch limitName {
	case "max_raw_bytes":
		actual = metadata.OriginalByteSize
		limit = options.MaxRawBytes
	case "max_provider_bytes":
		limit = options.MaxProviderBytes
	case "max_long_edge":
		longEdge := metadata.ProviderWidth
		if metadata.ProviderHeight > longEdge {
			longEdge = metadata.ProviderHeight
		}
		actual = int64(longEdge)
		limit = int64(options.MaxLongEdge)
	case "max_pixels":
		actual = int64(metadata.ProviderWidth) * int64(metadata.ProviderHeight)
		limit = options.MaxPixels
	case "max_encoded_image_bytes":
		actual = int64(base64.StdEncoding.EncodedLen(int(metadata.ProviderByteSize)))
		limit = options.MaxEncodedBytes
	}
	return newCaptureError(CodeProviderPayloadTooLarge, "prepare_semantic_image", fmt.Sprintf("semantic diff image exceeds %s budget", limitName), nil).
		WithMetadata("limit_name", limitName).
		WithMetadata("limit_value", limit).
		WithMetadata("actual_value", actual).
		WithMetadata("path", metadata.OriginalPath)
}

func semanticImageLimits(options semanticImagePrepareOptions) map[string]any {
	limits := map[string]any{}
	if options.MaxRawBytes > 0 {
		limits["max_raw_bytes"] = options.MaxRawBytes
	}
	if options.MaxProviderBytes > 0 {
		limits["max_provider_bytes"] = options.MaxProviderBytes
	}
	if options.MaxLongEdge > 0 {
		limits["max_long_edge"] = options.MaxLongEdge
	}
	if options.MaxPixels > 0 {
		limits["max_pixels"] = options.MaxPixels
	}
	if options.MaxEncodedBytes > 0 {
		limits["max_encoded_image_bytes"] = options.MaxEncodedBytes
	}
	if len(limits) == 0 {
		return nil
	}
	return limits
}

func resizeSemanticImage(payload []byte, mimeType string, options semanticImagePrepareOptions) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	width := src.Bounds().Dx()
	height := src.Bounds().Dy()
	for range 8 {
		targetWidth, targetHeight := semanticResizeDimensions(width, height, int64(len(payload)), options)
		if targetWidth >= width && targetHeight >= height {
			targetWidth = maxInt(1, width*85/100)
			targetHeight = maxInt(1, height*85/100)
		}
		dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
		for y := 0; y < targetHeight; y++ {
			sy := src.Bounds().Min.Y + y*height/targetHeight
			for x := 0; x < targetWidth; x++ {
				sx := src.Bounds().Min.X + x*width/targetWidth
				dst.Set(x, y, src.At(sx, sy))
			}
		}
		var out bytes.Buffer
		writer := writerFunc(func(p []byte) (int, error) { return out.Write(p) })
		switch mimeType {
		case "image/jpeg":
			err = jpeg.Encode(writer, dst, &jpeg.Options{Quality: 85})
		default:
			err = png.Encode(writer, dst)
		}
		if err != nil {
			return nil, err
		}
		payload = out.Bytes()
		width, height = targetWidth, targetHeight
		check := SemanticImageMetadata{ProviderByteSize: int64(len(payload)), ProviderWidth: width, ProviderHeight: height}
		if semanticResizeReason(check, options) == "" {
			return payload, nil
		}
		src = dst
	}
	return payload, nil
}

type writerFunc func([]byte) (int, error)

func (fn writerFunc) Write(p []byte) (int, error) { return fn(p) }

func semanticResizeDimensions(width, height int, byteSize int64, options semanticImagePrepareOptions) (int, int) {
	scale := 1.0
	longEdge := width
	if height > longEdge {
		longEdge = height
	}
	if options.MaxLongEdge > 0 && longEdge > options.MaxLongEdge {
		scale = minFloat(scale, float64(options.MaxLongEdge)/float64(longEdge))
	}
	pixels := int64(width) * int64(height)
	if options.MaxPixels > 0 && pixels > options.MaxPixels {
		scale = minFloat(scale, sqrtFloat(float64(options.MaxPixels)/float64(pixels)))
	}
	byteLimit := options.MaxProviderBytes
	if byteLimit <= 0 {
		byteLimit = options.MaxRawBytes
	}
	if byteLimit > 0 && byteSize > byteLimit {
		scale = minFloat(scale, sqrtFloat(float64(byteLimit)/float64(byteSize))*0.95)
	}
	if options.MaxEncodedBytes > 0 {
		encodedBytes := int64(base64.StdEncoding.EncodedLen(int(byteSize)))
		if encodedBytes > options.MaxEncodedBytes {
			scale = minFloat(scale, sqrtFloat(float64(options.MaxEncodedBytes)/float64(encodedBytes))*0.95)
		}
	}
	if options.ScaleFactor > 0 && options.ScaleFactor < 1 {
		scale = minFloat(scale, options.ScaleFactor)
	}
	return maxInt(1, int(float64(width)*scale)), maxInt(1, int(float64(height)*scale))
}

func semanticProviderCopyName(role, path, mimeType string) string {
	ext := ".png"
	if mimeType == "image/jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name := strings.Trim(strings.ToLower(role+"-"+base), ". ")
	if name == "" {
		name = "image"
	}
	return name + ext
}

func minFloat(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func sqrtFloat(value float64) float64 {
	if value <= 0 {
		return 1
	}
	x := value
	for range 8 {
		x = 0.5 * (x + value/x)
	}
	return x
}

func validateSemanticEncodedBudgets(images []SemanticImagePayload, maxEncodedImageBytes, maxCombinedEncodedBytes int64) error {
	var combined int64
	for _, image := range images {
		encodedBytes := int64(len(image.Base64Data))
		if maxEncodedImageBytes > 0 && encodedBytes > maxEncodedImageBytes {
			return semanticEncodedBudgetError("max_encoded_image_bytes", maxEncodedImageBytes, encodedBytes)
		}
		combined += encodedBytes
	}
	if maxCombinedEncodedBytes > 0 && combined > maxCombinedEncodedBytes {
		return semanticEncodedBudgetError("max_combined_encoded_image_bytes", maxCombinedEncodedBytes, combined)
	}
	return nil
}

func semanticEncodedBudgetError(limitName string, limit, actual int64) error {
	return newCaptureError(CodeProviderPayloadTooLarge, "prepare_semantic_image", "semantic diff provider payload exceeds encoded image budget", nil).
		WithMetadata("limit_name", limitName).
		WithMetadata("limit_value", limit).
		WithMetadata("actual_value", actual)
}

func semanticImageMetadata(images []SemanticImagePayload) []SemanticImageMetadata {
	out := make([]SemanticImageMetadata, 0, len(images))
	for _, image := range images {
		metadata := image.Metadata
		if metadata.Role == "" {
			continue
		}
		if len(metadata.Limits) > 0 {
			keys := make([]string, 0, len(metadata.Limits))
			for key := range metadata.Limits {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			limits := make(map[string]any, len(metadata.Limits))
			for _, key := range keys {
				limits[key] = metadata.Limits[key]
			}
			metadata.Limits = limits
		}
		out = append(out, metadata)
	}
	return out
}
