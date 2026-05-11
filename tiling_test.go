package webcap

import "testing"

func TestPlanTilesExactMultipleAndRemainder(t *testing.T) {
	tiling, err := planTiles(Bounds{Width: 250, Height: 120}, CaptureTileOptions{
		MaxWidth:          100,
		MaxHeight:         50,
		MaxPixels:         10000,
		MaxTargetPixels:   100000,
		MaxStitchedPixels: 100000,
	}, 1)
	if err != nil {
		t.Fatalf("planTiles returned error: %v", err)
	}
	if tiling.TileCount != 9 {
		t.Fatalf("unexpected tile count: %d", tiling.TileCount)
	}
	last := tiling.Tiles[len(tiling.Tiles)-1]
	if last.Row != 2 || last.Column != 2 || last.SourceBounds.Width != 50 || last.SourceBounds.Height != 20 {
		t.Fatalf("unexpected last tile: %+v", last)
	}
}

func TestPlanTilesNormalizesFractionalBounds(t *testing.T) {
	tiling, err := planTiles(Bounds{X: 1.2, Y: 2.8, Width: 10.1, Height: 12.2}, CaptureTileOptions{}, 1)
	if err != nil {
		t.Fatalf("planTiles returned error: %v", err)
	}
	target := tiling.TargetBounds
	if target.X != 1 || target.Y != 2 || target.Width != 11 || target.Height != 13 {
		t.Fatalf("unexpected normalized target: %+v", target)
	}
}

func TestPlanTilesOverlapCropsDestinationBounds(t *testing.T) {
	tiling, err := planTiles(Bounds{Width: 180, Height: 80}, CaptureTileOptions{
		MaxWidth:          100,
		MaxHeight:         80,
		MaxPixels:         10000,
		MaxTargetPixels:   100000,
		MaxStitchedPixels: 100000,
		Overlap:           20,
	}, 1)
	if err != nil {
		t.Fatalf("planTiles returned error: %v", err)
	}
	if tiling.TileCount != 2 {
		t.Fatalf("unexpected tile count: %d", tiling.TileCount)
	}
	second := tiling.Tiles[1]
	if second.SourceBounds.X != 80 || second.SourceBounds.Width != 100 {
		t.Fatalf("unexpected source bounds: %+v", second.SourceBounds)
	}
	if second.DestinationBounds == nil || second.DestinationBounds.X != 100 || second.DestinationBounds.Width != 80 {
		t.Fatalf("unexpected destination bounds: %+v", second.DestinationBounds)
	}
}

func TestPlanTilesRejectsScaledTargetPixels(t *testing.T) {
	_, err := planTiles(Bounds{Width: 100, Height: 100}, CaptureTileOptions{
		MaxWidth:          100,
		MaxHeight:         100,
		MaxPixels:         100000,
		MaxTargetPixels:   39999,
		MaxStitchedPixels: 100000,
	}, 2)
	if err == nil {
		t.Fatal("expected scaled target pixel limit error")
	}
}

func TestTargetExceedsLimitsUsesDimensionsAndScale(t *testing.T) {
	limits := CaptureTileLimits{MaxWidth: 100, MaxHeight: 100, MaxTargetPixels: 10000, ScaleFactor: 1}
	if !targetExceedsLimits(Bounds{Width: 101, Height: 50}, limits) {
		t.Fatal("expected width limit to trigger")
	}
	limits = CaptureTileLimits{MaxWidth: 100, MaxHeight: 100, MaxTargetPixels: 39999, ScaleFactor: 2}
	if !targetExceedsLimits(Bounds{Width: 100, Height: 100}, limits) {
		t.Fatal("expected scaled pixel limit to trigger")
	}
}

func TestTargetExceedsLimitsUsesMaxPixelsBudget(t *testing.T) {
	limits := CaptureTileLimits{
		MaxWidth:        DefaultTileMaxWidth,
		MaxHeight:       DefaultTileMaxHeight,
		MaxPixels:       DefaultTileMaxPixels,
		MaxTargetPixels: DefaultTileMaxTargetPixels,
		ScaleFactor:     2,
	}
	target := Bounds{Width: 6000, Height: 3000}
	if !targetExceedsLimits(target, limits) {
		t.Fatal("expected max_pixels to trigger oversize preflight")
	}
}
