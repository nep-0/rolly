package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	webp "github.com/skrashevich/go-webp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"

	"rolly/internal/exporter"
	"rolly/internal/model"
	"rolly/internal/store"
)

type Server struct {
	store       *store.Store
	frontendDir string
	mux         *http.ServeMux
}

func NewServer(st *store.Store, frontendDir string) *Server {
	s := &Server{store: st, frontendDir: frontendDir, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.health)
	s.mux.Handle("/", http.FileServer(http.Dir(s.frontendDir)))
	s.mux.HandleFunc("/api/v1/film-models", s.handleFilmModels)
	s.mux.HandleFunc("/api/v1/film-models/", s.handleFilmModel)
	s.mux.HandleFunc("/api/v1/cameras", s.handleCameras)
	s.mux.HandleFunc("/api/v1/cameras/", s.handleCamera)
	s.mux.HandleFunc("/api/v1/film-stocks", s.handleStocks)
	s.mux.HandleFunc("/api/v1/film-stocks/", s.handleStockTree)
	s.mux.HandleFunc("/api/v1/images/", s.handleImageTree)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFilmModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListFilmModels()
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var in model.FilmModel
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		if in.ID == "" {
			in.ID = uuid.NewString()
		}
		out, err := s.store.CreateFilmModel(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleFilmModel(w http.ResponseWriter, r *http.Request) {
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		out, err := s.store.GetFilmModel(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPut, http.MethodPatch:
		var in model.FilmModel
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		in.ID = id
		out, err := s.store.UpdateFilmModel(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		if err := s.store.DeleteFilmModel(id); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListCameras()
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var in cameraPayload
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		if in.ID == "" {
			in.ID = uuid.NewString()
		}
		out, err := s.store.CreateCamera(in.Camera())
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleCamera(w http.ResponseWriter, r *http.Request) {
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		out, err := s.store.GetCamera(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPut, http.MethodPatch:
		var in cameraPayload
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		in.ID = id
		out, err := s.store.UpdateCamera(in.Camera())
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		if err := s.store.DeleteCamera(id); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleStocks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListStocks()
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var in model.FilmStock
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		if in.ID == "" {
			in.ID = uuid.NewString()
		}
		out, err := s.store.CreateStock(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleStockTree(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/film-stocks/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	stockID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			stock, modelRow, camera, ranges, images, err := s.store.StockDetail(stockID)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"stock":  stock,
				"model":  modelRow,
				"camera": camera,
				"ranges": ranges,
				"images": images,
			})
		case http.MethodPut, http.MethodPatch:
			var in model.FilmStock
			if err := decodeJSON(r.Body, &in); err != nil {
				writeErr(w, err)
				return
			}
			in.ID = stockID
			out, err := s.store.UpdateStock(in)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		case http.MethodDelete:
			if err := s.store.DeleteStock(stockID); err != nil {
				writeErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
		return
	}
	switch parts[1] {
	case "images":
		s.handleStockImages(stockID, w, r, parts[2:])
	case "ranges":
		s.handleStockRanges(stockID, w, r, parts[2:])
	case "exports":
		s.handleStockExport(stockID, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleStockImages(stockID string, w http.ResponseWriter, r *http.Request, tail []string) {
	if len(tail) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := s.store.ListImages(stockID)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			if err := r.ParseMultipartForm(64 << 20); err != nil {
				writeErr(w, err)
				return
			}
			files := r.MultipartForm.File["files"]
			if len(files) == 0 {
				writeErr(w, fmt.Errorf("missing files"))
				return
			}
			created := make([]model.Image, 0, len(files))
			for _, fh := range files {
				f, err := fh.Open()
				if err != nil {
					writeErr(w, err)
					return
				}
				data, err := io.ReadAll(f)
				_ = f.Close()
				if err != nil {
					writeErr(w, err)
					return
				}
				storedName := uuid.NewString() + "_" + sanitizeFilename(fh.Filename)
				storedPath := filepath.Join(s.store.UploadDir(), stockID, storedName)
				if err := os.MkdirAll(filepath.Dir(storedPath), 0o755); err != nil {
					writeErr(w, err)
					return
				}
				if err := os.WriteFile(storedPath, data, 0o644); err != nil {
					writeErr(w, err)
					return
				}
				orderIndex, err := s.store.NextImageOrderIndex(stockID)
				if err != nil {
					writeErr(w, err)
					return
				}
				img := model.Image{
					ID:           uuid.NewString(),
					StockID:      stockID,
					OriginalName: fh.Filename,
					StoredName:   storedName,
					StoredPath:   storedPath,
					ContentType:  fh.Header.Get("Content-Type"),
					OrderIndex:   orderIndex,
				}
				out, err := s.store.CreateImage(img)
				if err != nil {
					writeErr(w, err)
					return
				}
				created = append(created, out)
			}
			writeJSON(w, http.StatusCreated, created)
		case http.MethodPatch:
			var in struct {
				ImageIDs []string `json:"image_ids"`
			}
			if err := decodeJSON(r.Body, &in); err != nil {
				writeErr(w, err)
				return
			}
			if err := s.store.ReorderImages(stockID, in.ImageIDs); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(tail) == 1 && tail[0] == "reorder" && r.Method == http.MethodPatch {
		var in struct {
			ImageIDs []string `json:"image_ids"`
		}
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		if err := s.store.ReorderImages(stockID, in.ImageIDs); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleStockRanges(stockID string, w http.ResponseWriter, r *http.Request, tail []string) {
	if len(tail) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := s.store.ListRanges(stockID)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var in model.FrameRange
			if err := decodeJSON(r.Body, &in); err != nil {
				writeErr(w, err)
				return
			}
			if in.ID == "" {
				in.ID = uuid.NewString()
			}
			in.StockID = stockID
			out, err := s.store.CreateRange(in)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		default:
			http.NotFound(w, r)
		}
		return
	}
	id := tail[0]
	switch r.Method {
	case http.MethodGet:
		out, err := s.store.GetRange(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPatch, http.MethodPut:
		var in model.FrameRange
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		in.ID = id
		in.StockID = stockID
		out, err := s.store.UpdateRange(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		if err := s.store.DeleteRange(id); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleStockExport(stockID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	stock, modelRow, camera, ranges, images, err := s.store.StockDetail(stockID)
	if err != nil {
		writeErr(w, err)
		return
	}
	res, err := exporter.Export(exporter.Source{
		Stock:  stock,
		Model:  modelRow,
		Camera: camera,
		Ranges: ranges,
		Images: images,
	}, s.store.ExportDir())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleImageTree(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/images/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		if err := s.store.DeleteImage(id); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch parts[1] {
	case "content":
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		img, err := s.store.GetImage(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		if err := writeWebPPreview(w, img.StoredPath); err != nil {
			writeErr(w, err)
			return
		}
	case "range":
		if r.Method != http.MethodPatch {
			http.NotFound(w, r)
			return
		}
		var in struct {
			RangeID string `json:"range_id"`
		}
		if err := decodeJSON(r.Body, &in); err != nil {
			writeErr(w, err)
			return
		}
		if err := s.store.AssignImageRange(id, in.RangeID); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func writeWebPPreview(w http.ResponseWriter, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return seekErr
		}
		src, err = tiff.Decode(f)
		if err != nil {
			return err
		}
	}
	thumb := resizeForPreview(src, 420)
	var buf bytes.Buffer
	if err := webp.Encode(&buf, thumb, &webp.Options{Lossy: true, Quality: 72}); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, err = w.Write(buf.Bytes())
	return err
}

func resizeForPreview(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	width := b.Dx()
	height := b.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	if width <= maxSide && height <= maxSide {
		return src
	}
	dstW, dstH := maxSide, maxSide
	if width >= height {
		dstH = maxSide * height / width
	} else {
		dstW = maxSide * width / height
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func decodeJSON(r io.Reader, dst any) error {
	return json.NewDecoder(r).Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("\\", "_", "/", "_", ":", "_", " ", "_").Replace(name)
	if name == "" {
		return "upload.jpg"
	}
	return name
}

type cameraPayload struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Maker         string  `json:"maker"`
	Model         string  `json:"model"`
	SerialNumber  string  `json:"serial_number"`
	MeteringMode  string  `json:"metering_mode"`
	FocalLength   numeric `json:"focal_length"`
	FocalLength35 numeric `json:"focal_length_35mm"`
}

func (p cameraPayload) Camera() model.Camera {
	return model.Camera{
		ID:              p.ID,
		Name:            p.Name,
		Maker:           p.Maker,
		Model:           p.Model,
		SerialNumber:    p.SerialNumber,
		MeteringMode:    p.MeteringMode,
		FocalLength:     float64(p.FocalLength),
		FocalLength35mm: float64(p.FocalLength35),
	}
}

type numeric float64

func (n *numeric) UnmarshalJSON(data []byte) error {
	var asFloat float64
	if err := json.Unmarshal(data, &asFloat); err == nil {
		*n = numeric(asFloat)
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		if asString == "" {
			*n = 0
			return nil
		}
		var parsed float64
		if _, err := fmt.Sscanf(asString, "%f", &parsed); err != nil {
			return err
		}
		*n = numeric(parsed)
		return nil
	}
	return fmt.Errorf("invalid numeric value")
}
