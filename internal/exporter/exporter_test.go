package exporter

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rolly/internal/model"
)

func TestDeriveFrameNumbersUsesRangeLocalOrder(t *testing.T) {
	rangeID := "range-1"
	created := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	src := Source{
		Ranges: []model.FrameRange{
			{ID: rangeID, StartFrame: 10, EndFrame: 12},
		},
		Images: []model.Image{
			{ID: "before", OrderIndex: 0, CreatedAt: created},
			{ID: "a", OrderIndex: 3, RangeID: &rangeID, CreatedAt: created.Add(time.Second)},
			{ID: "b", OrderIndex: 4, RangeID: &rangeID, CreatedAt: created.Add(2 * time.Second)},
			{ID: "c", OrderIndex: 5, RangeID: &rangeID, CreatedAt: created.Add(3 * time.Second)},
		},
	}
	byRange := map[string]model.FrameRange{rangeID: src.Ranges[0]}

	got := deriveFrameNumbers(src, byRange)
	if got["a"] != 10 || got["b"] != 11 || got["c"] != 12 {
		t.Fatalf("range-local frame numbers = a:%d b:%d c:%d, want 10, 11, 12", got["a"], got["b"], got["c"])
	}
	if got["before"] != 0 {
		t.Fatalf("unassigned frame number = %d, want stock order index 0", got["before"])
	}
}

func TestDeriveExportIndexesUsesRangeLocalOneBasedOrder(t *testing.T) {
	rangeID := "range-1"
	created := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	src := Source{
		Images: []model.Image{
			{ID: "before", OrderIndex: 0, CreatedAt: created},
			{ID: "a", OrderIndex: 3, RangeID: &rangeID, CreatedAt: created.Add(time.Second)},
			{ID: "b", OrderIndex: 4, RangeID: &rangeID, CreatedAt: created.Add(2 * time.Second)},
		},
	}

	got := deriveExportIndexes(src)
	if got["before"] != 1 || got["a"] != 1 || got["b"] != 2 {
		t.Fatalf("export indexes = before:%d a:%d b:%d, want 1, 1, 2", got["before"], got["a"], got["b"])
	}
}

func TestExportNamesIncludeFrameAndRangeLocalIndex(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "in.jpg")
	if err := os.WriteFile(input, tinyJPEG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	rangeID := "range-1"
	created := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	res, err := Export(Source{
		Stock:  model.FilmStock{ID: "stk-1111", ScannerModel: "scanner"},
		Model:  model.FilmModel{Name: "Film", ISO: 400},
		Camera: model.Camera{Name: "Camera"},
		Ranges: []model.FrameRange{
			{ID: "earlier", StartFrame: 0, EndFrame: 0},
			{ID: rangeID, StartFrame: 10, EndFrame: 10},
		},
		Images: []model.Image{
			{ID: "a", OriginalName: "a.jpg", StoredPath: input, OrderIndex: 0, RangeID: &rangeID, CreatedAt: created},
			{ID: "b", OriginalName: "b.jpg", StoredPath: input, OrderIndex: 1, RangeID: &rangeID, CreatedAt: created.Add(time.Second)},
		},
	}, filepath.Join(tmp, "exports"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 2 {
		t.Fatalf("exported files = %d, want 2", len(res.Files))
	}
	if res.Files[0].OutputName != "stk-1111-0002-0001.jpg" {
		t.Fatalf("first output name = %q", res.Files[0].OutputName)
	}
	if res.Files[1].OutputName != "stk-1111-0002-0002.jpg" {
		t.Fatalf("second output name = %q", res.Files[1].OutputName)
	}
	if res.Files[0].FrameNumber != 10 || res.Files[1].FrameNumber != 10 {
		t.Fatalf("frame numbers = %d, %d, want 10, 10", res.Files[0].FrameNumber, res.Files[1].FrameNumber)
	}
	if res.Files[0].OutputPath == res.Files[1].OutputPath {
		t.Fatal("exports collided on the same output path")
	}
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
