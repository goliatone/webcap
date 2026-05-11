package webcap

import (
	"math"
)

func tileLimits(req CaptureRequest) CaptureTileLimits {
	tile := effectiveTileOptions(req.Tile)
	scale := req.Viewport.ScaleFactor
	if scale <= 0 {
		scale = defaultScaleFactor
	}
	return CaptureTileLimits{
		MaxWidth:          tile.MaxWidth,
		MaxHeight:         tile.MaxHeight,
		MaxPixels:         tile.MaxPixels,
		MaxTargetPixels:   tile.MaxTargetPixels,
		MaxStitchedPixels: tile.MaxStitchedPixels,
		Overlap:           tile.Overlap,
		ScaleFactor:       scale,
	}
}

func normalizeTileTarget(bounds Bounds) Bounds {
	left := math.Floor(bounds.X)
	top := math.Floor(bounds.Y)
	right := math.Ceil(bounds.X + bounds.Width)
	bottom := math.Ceil(bounds.Y + bounds.Height)
	if right <= left {
		right = left + 1
	}
	if bottom <= top {
		bottom = top + 1
	}
	return Bounds{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func scaledPixels(width, height, scale float64) int64 {
	if scale <= 0 {
		scale = defaultScaleFactor
	}
	return int64(math.Ceil(width * height * scale * scale))
}

func targetExceedsLimits(target Bounds, limits CaptureTileLimits) bool {
	if target.Width > float64(limits.MaxWidth) || target.Height > float64(limits.MaxHeight) {
		return true
	}
	targetPixels := scaledPixels(target.Width, target.Height, limits.ScaleFactor)
	return targetPixels > limits.MaxPixels || targetPixels > limits.MaxTargetPixels
}

func planTiles(target Bounds, options CaptureTileOptions, scale float64) (*CaptureTiling, error) {
	options = effectiveTileOptions(options)
	target = normalizeTileTarget(target)
	limits := CaptureTileLimits{
		MaxWidth:          options.MaxWidth,
		MaxHeight:         options.MaxHeight,
		MaxPixels:         options.MaxPixels,
		MaxTargetPixels:   options.MaxTargetPixels,
		MaxStitchedPixels: options.MaxStitchedPixels,
		Overlap:           options.Overlap,
		ScaleFactor:       scale,
	}
	if scaledPixels(target.Width, target.Height, scale) > limits.MaxTargetPixels {
		return nil, newOversizeError("plan_tiles", "", target, limits, OversizePolicyTile)
	}

	stepX := float64(options.MaxWidth - options.Overlap)
	stepY := float64(options.MaxHeight - options.Overlap)
	if stepX <= 0 || stepY <= 0 {
		return nil, newCaptureError(CodeValidation, "plan_tiles", "tile overlap must leave positive tile steps", nil)
	}

	var tiles []CaptureTile
	index := 0
	for row, y := 0, target.Y; y < target.Y+target.Height; row, y = row+1, y+stepY {
		sourceTop := y
		sourceBottom := math.Min(target.Y+target.Height, y+float64(options.MaxHeight))
		for column, x := 0, target.X; x < target.X+target.Width; column, x = column+1, x+stepX {
			sourceLeft := x
			sourceRight := math.Min(target.X+target.Width, x+float64(options.MaxWidth))
			source := Bounds{
				X:      sourceLeft,
				Y:      sourceTop,
				Width:  sourceRight - sourceLeft,
				Height: sourceBottom - sourceTop,
			}
			if source.Width <= 0 || source.Height <= 0 {
				return nil, newCaptureError(CodeValidation, "plan_tiles", "planned tile has non-positive bounds", nil)
			}
			if scaledPixels(source.Width, source.Height, scale) > limits.MaxPixels {
				return nil, newCaptureError(CodeOversize, "plan_tiles", "planned tile exceeds max_pixels", nil).
					WithMetadata("tile_bounds", source).
					WithMetadata("limits", limits)
			}
			dest := Bounds{
				X:      math.Max(sourceLeft, target.X) - target.X,
				Y:      math.Max(sourceTop, target.Y) - target.Y,
				Width:  source.Width,
				Height: source.Height,
			}
			if options.Overlap > 0 {
				if column > 0 {
					dest.X += float64(options.Overlap)
					dest.Width -= float64(options.Overlap)
				}
				if row > 0 {
					dest.Y += float64(options.Overlap)
					dest.Height -= float64(options.Overlap)
				}
			}
			if dest.Width <= 0 || dest.Height <= 0 {
				return nil, newCaptureError(CodeValidation, "plan_tiles", "planned tile has non-positive stitched destination bounds", nil)
			}
			tiles = append(tiles, CaptureTile{
				Index:             index,
				Row:               row,
				Column:            column,
				SourceBounds:      source,
				DestinationBounds: &dest,
				Status:            CaptureTilePending,
			})
			index++
			if sourceRight >= target.X+target.Width {
				break
			}
		}
		if sourceBottom >= target.Y+target.Height {
			break
		}
	}

	return &CaptureTiling{
		Status:       CaptureTilingComplete,
		TargetBounds: target,
		Limits:       limits,
		TileCount:    len(tiles),
		Tiles:        tiles,
	}, nil
}
