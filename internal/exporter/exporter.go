package exporter

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	exifundefined "github.com/dsoprea/go-exif/v3/undefined"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
	tiffstructure "github.com/dsoprea/go-tiff-image-structure/v2"
	"golang.org/x/image/tiff"

	"rolly/internal/model"
)

type Source struct {
	Stock  model.FilmStock
	Model  model.FilmModel
	Camera model.Camera
	Ranges []model.FrameRange
	Images []model.Image
}

func Export(stock Source, exportDir string) (model.ExportResult, error) {
	outDir := filepath.Join(exportDir, sanitize(stock.Stock.ID))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return model.ExportResult{}, err
	}
	byRange := map[string]model.FrameRange{}
	for _, r := range stock.Ranges {
		byRange[r.ID] = r
	}
	sort.Slice(stock.Images, func(i, j int) bool {
		if stock.Images[i].OrderIndex == stock.Images[j].OrderIndex {
			return stock.Images[i].CreatedAt.Before(stock.Images[j].CreatedAt)
		}
		return stock.Images[i].OrderIndex < stock.Images[j].OrderIndex
	})
	takenTimes := deriveTakenTimes(stock, byRange)
	frameNumbers := deriveFrameNumbers(stock, byRange)
	rangeIndexes := deriveRangeIndexes(stock.Ranges)
	exportIndexes := deriveExportIndexes(stock)
	files := make([]model.ExportFile, 0, len(stock.Images))
	for _, img := range stock.Images {
		in, err := os.ReadFile(img.StoredPath)
		if err != nil {
			return model.ExportResult{}, err
		}
		rangeMeta := ""
		frameNumber := frameNumbers[img.ID]
		if img.RangeID != nil {
			if r, ok := byRange[*img.RangeID]; ok {
				rangeMeta = fmt.Sprintf("frames %d-%d; ", r.StartFrame, r.EndFrame)
				if r.Location != "" {
					rangeMeta += "location=" + r.Location + "; "
				}
				if r.Weather != "" {
					rangeMeta += "weather=" + r.Weather + "; "
				}
				if r.Notes != "" {
					rangeMeta += "notes=" + r.Notes + "; "
				}
			}
		}
		outExt := outputExt(in, img.OriginalName)
		outName := fmt.Sprintf("%s-%04d-%04d%s", sanitize(stock.Stock.ID), rangeIndexes[imageRangeID(img)], exportIndexes[img.ID], outExt)
		outPath := filepath.Join(outDir, outName)
		rendered, err := rewriteExif(in, stock, img, frameNumber, rangeMeta, takenTimes[img.ID])
		if err != nil {
			return model.ExportResult{}, err
		}
		if err := os.WriteFile(outPath, rendered, 0o644); err != nil {
			return model.ExportResult{}, err
		}
		files = append(files, model.ExportFile{
			ImageID:     img.ID,
			SourcePath:  img.StoredPath,
			OutputPath:  outPath,
			OutputName:  outName,
			FrameNumber: frameNumber,
		})
	}
	manifest := filepath.Join(outDir, "manifest.json")
	manifestBytes, _ := json.MarshalIndent(files, "", "  ")
	if err := os.WriteFile(manifest, manifestBytes, 0o644); err != nil {
		return model.ExportResult{}, err
	}
	return model.ExportResult{StockID: stock.Stock.ID, OutputDir: outDir, Manifest: manifest, Files: files}, nil
}

func imageRangeID(img model.Image) string {
	if img.RangeID == nil {
		return ""
	}
	return *img.RangeID
}

func deriveRangeIndexes(ranges []model.FrameRange) map[string]int {
	ordered := append([]model.FrameRange(nil), ranges...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].StartFrame == ordered[j].StartFrame {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].StartFrame < ordered[j].StartFrame
	})
	out := map[string]int{"": 0}
	for idx, r := range ordered {
		out[r.ID] = idx + 1
	}
	return out
}

func deriveExportIndexes(stock Source) map[string]int {
	out := make(map[string]int, len(stock.Images))
	grouped := make(map[string][]model.Image)
	for _, img := range stock.Images {
		key := ""
		if img.RangeID != nil {
			key = *img.RangeID
		}
		grouped[key] = append(grouped[key], img)
	}
	for _, images := range grouped {
		sort.Slice(images, func(i, j int) bool {
			if images[i].OrderIndex == images[j].OrderIndex {
				return images[i].CreatedAt.Before(images[j].CreatedAt)
			}
			return images[i].OrderIndex < images[j].OrderIndex
		})
		for idx, img := range images {
			out[img.ID] = idx + 1
		}
	}
	return out
}

func deriveFrameNumbers(stock Source, byRange map[string]model.FrameRange) map[string]int {
	out := make(map[string]int, len(stock.Images))
	grouped := make(map[string][]model.Image)
	for _, img := range stock.Images {
		out[img.ID] = img.OrderIndex
		if img.RangeID != nil {
			grouped[*img.RangeID] = append(grouped[*img.RangeID], img)
		}
	}
	for rangeID, images := range grouped {
		r, ok := byRange[rangeID]
		if !ok {
			continue
		}
		sort.Slice(images, func(i, j int) bool {
			if images[i].OrderIndex == images[j].OrderIndex {
				return images[i].CreatedAt.Before(images[j].CreatedAt)
			}
			return images[i].OrderIndex < images[j].OrderIndex
		})
		for idx, img := range images {
			frameNumber := r.StartFrame + idx
			if frameNumber > r.EndFrame {
				frameNumber = r.EndFrame
			}
			out[img.ID] = frameNumber
		}
	}
	return out
}

func rewriteExif(input []byte, stock Source, img model.Image, frameNumber int, rangeMeta string, takenAt time.Time) ([]byte, error) {
	if jpegstructure.NewJpegMediaParser().LooksLikeFormat(input) {
		return rewriteJPEGExif(input, stock, img, frameNumber, rangeMeta, takenAt)
	}
	if tiffstructure.NewTiffMediaParser().LooksLikeFormat(input) {
		return rewriteTIFFExif(input, stock, img, frameNumber, rangeMeta, takenAt)
	}
	return nil, fmt.Errorf("unsupported image format for EXIF export: %s", img.OriginalName)
}

func rewriteJPEGExif(input []byte, stock Source, img model.Image, frameNumber int, rangeMeta string, takenAt time.Time) ([]byte, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseBytes(input)
	if err != nil {
		return nil, err
	}
	sl := intfc.(*jpegstructure.SegmentList)
	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, err
	}
	ti := exif.NewTagIndex()
	rootIb := exif.NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.EncodeDefaultByteOrder)
	if stock.Camera.Maker != "" {
		_ = rootIb.SetStandardWithName("Make", stock.Camera.Maker)
	}
	if stock.Camera.Model != "" {
		_ = rootIb.SetStandardWithName("Model", stock.Camera.Model)
	}
	_ = rootIb.SetStandardWithName("Software", "Rolly")
	dateTaken := formatTakenAt(takenAt)
	_ = rootIb.SetStandardWithName("DateTime", dateTaken)
	exifIb, err := exif.GetOrCreateIbFromRootIb(rootIb, "IFD/Exif")
	if err != nil {
		return nil, err
	}
	_ = exifIb.SetStandardWithName("DateTimeOriginal", dateTaken)
	_ = exifIb.SetStandardWithName("DateTimeDigitized", dateTaken)
	_ = exifIb.SetStandardWithName("ISOSpeedRatings", []uint16{uint16(stock.Model.ISO)})
	_ = exifIb.SetStandardWithName("CameraSerialNumber", stock.Camera.SerialNumber)
	_ = exifIb.SetStandardWithName("BodySerialNumber", stock.Camera.SerialNumber)
	_ = exifIb.SetStandardWithName("MeteringMode", []uint16{meteringModeCode(stock.Camera.MeteringMode)})
	_ = exifIb.SetStandardWithName("FocalLength", []exifcommon.Rational{{Numerator: uint32(stock.Camera.FocalLength * 100), Denominator: 100}})
	_ = exifIb.SetStandardWithName("FocalLengthIn35mmFilm", []uint16{uint16(stock.Camera.FocalLength35mm)})
	comment := exifundefined.Tag9286UserComment{
		EncodingType:  exifundefined.TagUndefinedType_9286_UserComment_Encoding_ASCII,
		EncodingBytes: []byte(strings.TrimSpace(fmt.Sprintf("stock=%s model=%s camera=%s scanner=%s image=%s %s", stock.Stock.ID, stock.Model.Name, stock.Camera.Name, stock.Stock.ScannerModel, img.OriginalName, rangeMeta))),
	}
	if exifChild, err := exif.GetOrCreateIbFromRootIb(rootIb, "IFD/Exif"); err == nil {
		_ = exifChild.SetStandardWithName("UserComment", comment)
	}
	if err := sl.SetExif(rootIb); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := sl.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rewriteTIFFExif(input []byte, stock Source, img model.Image, frameNumber int, rangeMeta string, takenAt time.Time) ([]byte, error) {
	if _, err := tiffstructure.NewTiffMediaParser().ParseBytes(input); err != nil {
		return nil, err
	}
	decoded, err := tiff.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	rgba := toRGBA(decoded)
	meta := exifMeta{
		Make:             stock.Camera.Maker,
		Model:            stock.Camera.Model,
		Software:         "Rolly",
		DateTime:         formatTakenAt(takenAt),
		ISO:              stock.Model.ISO,
		MeteringMode:     meteringModeCode(stock.Camera.MeteringMode),
		FocalLength:      stock.Camera.FocalLength,
		FocalLength35mm:  stock.Camera.FocalLength35mm,
		BodySerialNumber: stock.Camera.SerialNumber,
		UserComment:      strings.TrimSpace(fmt.Sprintf("stock=%s model=%s camera=%s scanner=%s image=%s frame=%d %s", stock.Stock.ID, stock.Model.Name, stock.Camera.Name, stock.Stock.ScannerModel, img.OriginalName, frameNumber, rangeMeta)),
	}
	return encodeTIFFWithExif(rgba, meta)
}

func deriveTakenTimes(stock Source, byRange map[string]model.FrameRange) map[string]time.Time {
	out := make(map[string]time.Time, len(stock.Images))
	grouped := make(map[string][]model.Image)
	for _, img := range stock.Images {
		if img.RangeID != nil {
			grouped[*img.RangeID] = append(grouped[*img.RangeID], img)
		}
	}
	for rangeID, images := range grouped {
		r, ok := byRange[rangeID]
		if !ok || r.ShotFrom == nil {
			continue
		}
		sort.Slice(images, func(i, j int) bool {
			if images[i].OrderIndex == images[j].OrderIndex {
				return images[i].CreatedAt.Before(images[j].CreatedAt)
			}
			return images[i].OrderIndex < images[j].OrderIndex
		})
		for idx, img := range images {
			taken := r.ShotFrom.Add(time.Duration(idx) * time.Second)
			if r.ShotTo != nil && taken.After(*r.ShotTo) {
				taken = *r.ShotTo
			}
			out[img.ID] = taken
		}
	}
	return out
}

func formatTakenAt(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format("2006:01:02 15:04:05")
	}
	return t.UTC().Format("2006:01:02 15:04:05")
}

type exifMeta struct {
	Make             string
	Model            string
	Software         string
	DateTime         string
	ISO              int
	MeteringMode     uint16
	FocalLength      float64
	FocalLength35mm  float64
	BodySerialNumber string
	UserComment      string
}

type tiffEntry struct {
	tag   uint16
	typ   uint16
	count uint32
	data  []byte
}

func encodeTIFFWithExif(img *image.RGBA, meta exifMeta) ([]byte, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]byte, width*height*3)
	p := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		rowStart := (y - bounds.Min.Y) * img.Stride
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			i := rowStart + (x-bounds.Min.X)*4
			pixels[p] = img.Pix[i]
			pixels[p+1] = img.Pix[i+1]
			pixels[p+2] = img.Pix[i+2]
			p += 3
		}
	}

	root := []tiffEntry{
		longEntry(0x0100, uint32(width)),
		longEntry(0x0101, uint32(height)),
		shortsEntry(0x0102, []uint16{8, 8, 8}),
		shortEntry(0x0103, 1),
		shortEntry(0x0106, 2),
		asciiEntry(0x010f, meta.Make),
		asciiEntry(0x0110, meta.Model),
		longEntry(0x0111, 0),
		shortEntry(0x0115, 3),
		longEntry(0x0116, uint32(height)),
		longEntry(0x0117, uint32(len(pixels))),
		rationalEntry(0x011a, 72, 1),
		rationalEntry(0x011b, 72, 1),
		shortEntry(0x011c, 1),
		shortEntry(0x0128, 2),
		asciiEntry(0x0131, meta.Software),
		asciiEntry(0x0132, meta.DateTime),
		longEntry(0x8769, 0),
	}
	exifEntries := []tiffEntry{
		shortsEntry(0x8827, []uint16{uint16(meta.ISO)}),
		asciiEntry(0x9003, meta.DateTime),
		asciiEntry(0x9004, meta.DateTime),
		shortEntry(0x9207, meta.MeteringMode),
		rationalEntry(0x920a, uint32(meta.FocalLength*100), 100),
		undefinedEntry(0x9286, append([]byte{'A', 'S', 'C', 'I', 'I', 0, 0, 0}, []byte(meta.UserComment)...)),
		shortEntry(0xa405, uint16(meta.FocalLength35mm)),
		asciiEntry(0xa431, meta.BodySerialNumber),
	}

	rootIFDSize := ifdSize(len(root))
	rootExtraOffset := uint32(8 + rootIFDSize)
	rootExtraSize := extraSize(root)
	exifOffset := rootExtraOffset + rootExtraSize
	setLong(root, 0x8769, exifOffset)
	exifExtraOffset := exifOffset + uint32(ifdSize(len(exifEntries)))
	exifExtraSize := extraSize(exifEntries)
	pixelOffset := exifExtraOffset + exifExtraSize
	setLong(root, 0x0111, pixelOffset)

	var buf bytes.Buffer
	buf.Write([]byte{'I', 'I'})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(8))
	if err := writeIFD(&buf, root, rootExtraOffset); err != nil {
		return nil, err
	}
	if err := writeIFD(&buf, exifEntries, exifExtraOffset); err != nil {
		return nil, err
	}
	buf.Write(pixels)
	return buf.Bytes(), nil
}

func writeIFD(buf *bytes.Buffer, entries []tiffEntry, extraOffset uint32) error {
	sort.Slice(entries, func(i, j int) bool { return entries[i].tag < entries[j].tag })
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(entries)))
	extra := bytes.Buffer{}
	for _, e := range entries {
		_ = binary.Write(buf, binary.LittleEndian, e.tag)
		_ = binary.Write(buf, binary.LittleEndian, e.typ)
		_ = binary.Write(buf, binary.LittleEndian, e.count)
		if len(e.data) <= 4 {
			var inline [4]byte
			copy(inline[:], e.data)
			buf.Write(inline[:])
			continue
		}
		_ = binary.Write(buf, binary.LittleEndian, extraOffset+uint32(extra.Len()))
		extra.Write(e.data)
		if extra.Len()%2 != 0 {
			extra.WriteByte(0)
		}
	}
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	buf.Write(extra.Bytes())
	return nil
}

func ifdSize(n int) int {
	return 2 + n*12 + 4
}

func extraSize(entries []tiffEntry) uint32 {
	var total uint32
	for _, e := range entries {
		if len(e.data) > 4 {
			total += uint32(len(e.data))
			if total%2 != 0 {
				total++
			}
		}
	}
	return total
}

func setLong(entries []tiffEntry, tag uint16, value uint32) {
	for i := range entries {
		if entries[i].tag == tag {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], value)
			entries[i].data = b[:]
			entries[i].count = 1
			return
		}
	}
}

func asciiEntry(tag uint16, value string) tiffEntry {
	if value == "" {
		value = " "
	}
	return tiffEntry{tag: tag, typ: 2, count: uint32(len(value) + 1), data: append([]byte(value), 0)}
}

func undefinedEntry(tag uint16, value []byte) tiffEntry {
	return tiffEntry{tag: tag, typ: 7, count: uint32(len(value)), data: value}
}

func shortEntry(tag uint16, value uint16) tiffEntry {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], value)
	return tiffEntry{tag: tag, typ: 3, count: 1, data: b[:]}
}

func shortsEntry(tag uint16, values []uint16) tiffEntry {
	data := make([]byte, len(values)*2)
	for i, v := range values {
		binary.LittleEndian.PutUint16(data[i*2:], v)
	}
	return tiffEntry{tag: tag, typ: 3, count: uint32(len(values)), data: data}
}

func longEntry(tag uint16, value uint32) tiffEntry {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], value)
	return tiffEntry{tag: tag, typ: 4, count: 1, data: b[:]}
}

func rationalEntry(tag uint16, numerator, denominator uint32) tiffEntry {
	if denominator == 0 {
		denominator = 1
	}
	var b [8]byte
	binary.LittleEndian.PutUint32(b[0:], numerator)
	binary.LittleEndian.PutUint32(b[4:], denominator)
	return tiffEntry{tag: tag, typ: 5, count: 1, data: b[:]}
}

func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x-b.Min.X, y-b.Min.Y, color.RGBAModel.Convert(src.At(x, y)))
		}
	}
	return dst
}

func meteringModeCode(v string) uint16 {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "average":
		return 1
	case "center-weighted", "center weighted", "center-weighted average":
		return 2
	case "spot":
		return 3
	case "multi-spot":
		return 4
	case "pattern":
		return 5
	case "partial":
		return 6
	default:
		return 0
	}
}

func sanitize(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.NewReplacer("\\", "_", "/", "_", ":", "_", " ", "_").Replace(v)
	if v == "" {
		return "stock"
	}
	return v
}

func outputExt(data []byte, name string) string {
	if tiffstructure.NewTiffMediaParser().LooksLikeFormat(data) {
		return ".tiff"
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".jpeg" || ext == ".jpg" {
		return ".jpg"
	}
	return ".jpg"
}
