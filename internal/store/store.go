package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"rolly/internal/model"
)

type Store struct {
	db        *sql.DB
	uploadDir string
	exportDir string
}

func Open(dbPath, uploadDir, exportDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	st := &Store{db: db, uploadDir: uploadDir, exportDir: exportDir}
	if err := st.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) UploadDir() string { return s.uploadDir }
func (s *Store) ExportDir() string { return s.exportDir }

func (s *Store) migrate() error {
	schema := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS film_models (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			iso INTEGER NOT NULL,
			size TEXT NOT NULL,
			nominal_photo_count INTEGER,
			supported_processing_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS cameras (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			maker TEXT NOT NULL,
			model TEXT NOT NULL,
			serial_number TEXT NOT NULL,
			metering_mode TEXT NOT NULL,
			focal_length REAL NOT NULL,
			focal_length_35mm REAL NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS film_stocks (
			id TEXT PRIMARY KEY,
			model_id TEXT NOT NULL,
			camera_id TEXT NOT NULL,
			expiry_year INTEGER NOT NULL,
			expiry_month INTEGER NOT NULL,
			emulsion_number TEXT NOT NULL,
			chosen_processing TEXT NOT NULL,
			scanner_model TEXT NOT NULL,
			comment TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(model_id) REFERENCES film_models(id) ON DELETE RESTRICT,
			FOREIGN KEY(camera_id) REFERENCES cameras(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE IF NOT EXISTS frame_ranges (
			id TEXT PRIMARY KEY,
			stock_id TEXT NOT NULL,
			start_frame INTEGER NOT NULL,
			end_frame INTEGER NOT NULL,
			shot_from TEXT,
			shot_to TEXT,
			location TEXT NOT NULL DEFAULT '',
			weather TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(stock_id) REFERENCES film_stocks(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS images (
			id TEXT PRIMARY KEY,
			stock_id TEXT NOT NULL,
			original_name TEXT NOT NULL,
			stored_name TEXT NOT NULL,
			stored_path TEXT NOT NULL,
			content_type TEXT NOT NULL,
			order_index INTEGER NOT NULL DEFAULT 0,
			range_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(stock_id) REFERENCES film_stocks(id) ON DELETE CASCADE,
			FOREIGN KEY(range_id) REFERENCES frame_ranges(id) ON DELETE SET NULL
		)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range schema {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`ALTER TABLE film_stocks ADD COLUMN comment TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(v string) (time.Time, error) { return time.Parse(time.RFC3339Nano, v) }

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func scanFilmModel(row scanner, dst *model.FilmModel) error {
	var created, updated string
	var supported string
	var nominal sql.NullInt64
	if err := row.Scan(&dst.ID, &dst.Name, &dst.ISO, &dst.Size, &nominal, &supported, &created, &updated); err != nil {
		return err
	}
	if nominal.Valid {
		v := int(nominal.Int64)
		dst.NominalPhotoCount = &v
	}
	_ = json.Unmarshal([]byte(supported), &dst.SupportedProcessing)
	dst.CreatedAt, _ = parseTime(created)
	dst.UpdatedAt, _ = parseTime(updated)
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) CreateFilmModel(m model.FilmModel) (model.FilmModel, error) {
	if strings.TrimSpace(m.ID) == "" {
		return model.FilmModel{}, errors.New("missing id")
	}
	if m.SupportedProcessing == nil {
		m.SupportedProcessing = []string{}
	}
	ts := now()
	_, err := s.db.Exec(`INSERT INTO film_models(id, name, iso, size, nominal_photo_count, supported_processing_json, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.ISO, m.Size, m.NominalPhotoCount, mustJSON(m.SupportedProcessing), ts, ts)
	if err != nil {
		return model.FilmModel{}, err
	}
	m.CreatedAt, _ = parseTime(ts)
	m.UpdatedAt, _ = parseTime(ts)
	return m, nil
}

func (s *Store) ListFilmModels() ([]model.FilmModel, error) {
	rows, err := s.db.Query(`SELECT id,name,iso,size,nominal_photo_count,supported_processing_json,created_at,updated_at FROM film_models ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.FilmModel, 0)
	for rows.Next() {
		var m model.FilmModel
		if err := scanFilmModel(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetFilmModel(id string) (model.FilmModel, error) {
	row := s.db.QueryRow(`SELECT id,name,iso,size,nominal_photo_count,supported_processing_json,created_at,updated_at FROM film_models WHERE id=?`, id)
	var m model.FilmModel
	if err := scanFilmModel(row, &m); err != nil {
		return model.FilmModel{}, err
	}
	return m, nil
}

func (s *Store) UpdateFilmModel(m model.FilmModel) (model.FilmModel, error) {
	ts := now()
	_, err := s.db.Exec(`UPDATE film_models SET name=?, iso=?, size=?, nominal_photo_count=?, supported_processing_json=?, updated_at=? WHERE id=?`,
		m.Name, m.ISO, m.Size, m.NominalPhotoCount, mustJSON(m.SupportedProcessing), ts, m.ID)
	if err != nil {
		return model.FilmModel{}, err
	}
	return s.GetFilmModel(m.ID)
}

func (s *Store) DeleteFilmModel(id string) error {
	_, err := s.db.Exec(`DELETE FROM film_models WHERE id=?`, id)
	return err
}

func scanCamera(row scanner, dst *model.Camera) error {
	var created, updated string
	if err := row.Scan(&dst.ID, &dst.Name, &dst.Maker, &dst.Model, &dst.SerialNumber, &dst.MeteringMode, &dst.FocalLength, &dst.FocalLength35mm, &created, &updated); err != nil {
		return err
	}
	dst.CreatedAt, _ = parseTime(created)
	dst.UpdatedAt, _ = parseTime(updated)
	return nil
}

func (s *Store) CreateCamera(c model.Camera) (model.Camera, error) {
	ts := now()
	_, err := s.db.Exec(`INSERT INTO cameras(id,name,maker,model,serial_number,metering_mode,focal_length,focal_length_35mm,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.Name, c.Maker, c.Model, c.SerialNumber, c.MeteringMode, c.FocalLength, c.FocalLength35mm, ts, ts)
	if err != nil {
		return model.Camera{}, err
	}
	c.CreatedAt, _ = parseTime(ts)
	c.UpdatedAt, _ = parseTime(ts)
	return c, nil
}

func (s *Store) ListCameras() ([]model.Camera, error) {
	rows, err := s.db.Query(`SELECT id,name,maker,model,serial_number,metering_mode,focal_length,focal_length_35mm,created_at,updated_at FROM cameras ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Camera, 0)
	for rows.Next() {
		var c model.Camera
		if err := scanCamera(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCamera(id string) (model.Camera, error) {
	row := s.db.QueryRow(`SELECT id,name,maker,model,serial_number,metering_mode,focal_length,focal_length_35mm,created_at,updated_at FROM cameras WHERE id=?`, id)
	var c model.Camera
	if err := scanCamera(row, &c); err != nil {
		return model.Camera{}, err
	}
	return c, nil
}

func (s *Store) UpdateCamera(c model.Camera) (model.Camera, error) {
	ts := now()
	_, err := s.db.Exec(`UPDATE cameras SET name=?, maker=?, model=?, serial_number=?, metering_mode=?, focal_length=?, focal_length_35mm=?, updated_at=? WHERE id=?`,
		c.Name, c.Maker, c.Model, c.SerialNumber, c.MeteringMode, c.FocalLength, c.FocalLength35mm, ts, c.ID)
	if err != nil {
		return model.Camera{}, err
	}
	return s.GetCamera(c.ID)
}

func (s *Store) DeleteCamera(id string) error {
	_, err := s.db.Exec(`DELETE FROM cameras WHERE id=?`, id)
	return err
}

func scanStock(row scanner, dst *model.FilmStock) error {
	var created, updated string
	if err := row.Scan(&dst.ID, &dst.ModelID, &dst.CameraID, &dst.ExpiryYear, &dst.ExpiryMonth, &dst.EmulsionNumber, &dst.ChosenProcessing, &dst.ScannerModel, &dst.Comment, &created, &updated); err != nil {
		return err
	}
	dst.CreatedAt, _ = parseTime(created)
	dst.UpdatedAt, _ = parseTime(updated)
	return nil
}

func (s *Store) CreateStock(v model.FilmStock) (model.FilmStock, error) {
	ts := now()
	_, err := s.db.Exec(`INSERT INTO film_stocks(id,model_id,camera_id,expiry_year,expiry_month,emulsion_number,chosen_processing,scanner_model,comment,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.ModelID, v.CameraID, v.ExpiryYear, v.ExpiryMonth, v.EmulsionNumber, v.ChosenProcessing, v.ScannerModel, v.Comment, ts, ts)
	if err != nil {
		return model.FilmStock{}, err
	}
	v.CreatedAt, _ = parseTime(ts)
	v.UpdatedAt, _ = parseTime(ts)
	return v, nil
}

func (s *Store) ListStocks() ([]model.FilmStock, error) {
	rows, err := s.db.Query(`SELECT id,model_id,camera_id,expiry_year,expiry_month,emulsion_number,chosen_processing,scanner_model,comment,created_at,updated_at FROM film_stocks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.FilmStock, 0)
	for rows.Next() {
		var v model.FilmStock
		if err := scanStock(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetStock(id string) (model.FilmStock, error) {
	row := s.db.QueryRow(`SELECT id,model_id,camera_id,expiry_year,expiry_month,emulsion_number,chosen_processing,scanner_model,comment,created_at,updated_at FROM film_stocks WHERE id=?`, id)
	var v model.FilmStock
	if err := scanStock(row, &v); err != nil {
		return model.FilmStock{}, err
	}
	return v, nil
}

func (s *Store) UpdateStock(v model.FilmStock) (model.FilmStock, error) {
	ts := now()
	_, err := s.db.Exec(`UPDATE film_stocks SET model_id=?, camera_id=?, expiry_year=?, expiry_month=?, emulsion_number=?, chosen_processing=?, scanner_model=?, comment=?, updated_at=? WHERE id=?`,
		v.ModelID, v.CameraID, v.ExpiryYear, v.ExpiryMonth, v.EmulsionNumber, v.ChosenProcessing, v.ScannerModel, v.Comment, ts, v.ID)
	if err != nil {
		return model.FilmStock{}, err
	}
	return s.GetStock(v.ID)
}

func (s *Store) DeleteStock(id string) error {
	_, err := s.db.Exec(`DELETE FROM film_stocks WHERE id=?`, id)
	return err
}

func (s *Store) CreateRange(r model.FrameRange) (model.FrameRange, error) {
	if err := s.validateRange(r.StockID, r.ID, r.StartFrame, r.EndFrame); err != nil {
		return model.FrameRange{}, err
	}
	ts := now()
	_, err := s.db.Exec(`INSERT INTO frame_ranges(id,stock_id,start_frame,end_frame,shot_from,shot_to,location,weather,notes,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.StockID, r.StartFrame, r.EndFrame, timePtrString(r.ShotFrom), timePtrString(r.ShotTo), r.Location, r.Weather, r.Notes, ts, ts)
	if err != nil {
		return model.FrameRange{}, err
	}
	r.CreatedAt, _ = parseTime(ts)
	r.UpdatedAt, _ = parseTime(ts)
	return r, nil
}

func (s *Store) UpdateRange(r model.FrameRange) (model.FrameRange, error) {
	if err := s.validateRange(r.StockID, r.ID, r.StartFrame, r.EndFrame); err != nil {
		return model.FrameRange{}, err
	}
	ts := now()
	_, err := s.db.Exec(`UPDATE frame_ranges SET start_frame=?, end_frame=?, shot_from=?, shot_to=?, location=?, weather=?, notes=?, updated_at=? WHERE id=?`,
		r.StartFrame, r.EndFrame, timePtrString(r.ShotFrom), timePtrString(r.ShotTo), r.Location, r.Weather, r.Notes, ts, r.ID)
	if err != nil {
		return model.FrameRange{}, err
	}
	return s.GetRange(r.ID)
}

func (s *Store) validateRange(stockID, rangeID string, start, end int) error {
	if start < 0 || end < start {
		return fmt.Errorf("invalid frame range")
	}
	row := s.db.QueryRow(`SELECT COUNT(*) FROM frame_ranges WHERE stock_id=? AND id<>? AND NOT (? < start_frame OR ? > end_frame)`, stockID, rangeID, end, start)
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("frame range overlaps an existing range")
	}
	return nil
}

func (s *Store) DeleteRange(id string) error {
	_, err := s.db.Exec(`DELETE FROM frame_ranges WHERE id=?`, id)
	return err
}

func (s *Store) GetRange(id string) (model.FrameRange, error) {
	row := s.db.QueryRow(`SELECT id,stock_id,start_frame,end_frame,shot_from,shot_to,location,weather,notes,created_at,updated_at FROM frame_ranges WHERE id=?`, id)
	var r model.FrameRange
	var shotFrom, shotTo sql.NullString
	var created, updated string
	if err := row.Scan(&r.ID, &r.StockID, &r.StartFrame, &r.EndFrame, &shotFrom, &shotTo, &r.Location, &r.Weather, &r.Notes, &created, &updated); err != nil {
		return model.FrameRange{}, err
	}
	if shotFrom.Valid {
		t, _ := time.Parse(time.RFC3339Nano, shotFrom.String)
		r.ShotFrom = &t
	}
	if shotTo.Valid {
		t, _ := time.Parse(time.RFC3339Nano, shotTo.String)
		r.ShotTo = &t
	}
	r.CreatedAt, _ = parseTime(created)
	r.UpdatedAt, _ = parseTime(updated)
	return r, nil
}

func (s *Store) ListRanges(stockID string) ([]model.FrameRange, error) {
	rows, err := s.db.Query(`SELECT id,stock_id,start_frame,end_frame,shot_from,shot_to,location,weather,notes,created_at,updated_at FROM frame_ranges WHERE stock_id=? ORDER BY start_frame ASC`, stockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.FrameRange, 0)
	for rows.Next() {
		var r model.FrameRange
		var shotFrom, shotTo sql.NullString
		var created, updated string
		if err := rows.Scan(&r.ID, &r.StockID, &r.StartFrame, &r.EndFrame, &shotFrom, &shotTo, &r.Location, &r.Weather, &r.Notes, &created, &updated); err != nil {
			return nil, err
		}
		if shotFrom.Valid {
			t, _ := time.Parse(time.RFC3339Nano, shotFrom.String)
			r.ShotFrom = &t
		}
		if shotTo.Valid {
			t, _ := time.Parse(time.RFC3339Nano, shotTo.String)
			r.ShotTo = &t
		}
		r.CreatedAt, _ = parseTime(created)
		r.UpdatedAt, _ = parseTime(updated)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateImage(img model.Image) (model.Image, error) {
	ts := now()
	_, err := s.db.Exec(`INSERT INTO images(id,stock_id,original_name,stored_name,stored_path,content_type,order_index,range_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		img.ID, img.StockID, img.OriginalName, img.StoredName, img.StoredPath, img.ContentType, img.OrderIndex, nullableStringPtr(img.RangeID), ts, ts)
	if err != nil {
		return model.Image{}, err
	}
	img.CreatedAt, _ = parseTime(ts)
	img.UpdatedAt, _ = parseTime(ts)
	return img, nil
}

func (s *Store) NextImageOrderIndex(stockID string) (int, error) {
	row := s.db.QueryRow(`SELECT COALESCE(MAX(order_index), -1) + 1 FROM images WHERE stock_id=?`, stockID)
	var next int
	if err := row.Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *Store) ListImages(stockID string) ([]model.Image, error) {
	rows, err := s.db.Query(`SELECT id,stock_id,original_name,stored_name,stored_path,content_type,order_index,range_id,created_at,updated_at FROM images WHERE stock_id=? ORDER BY order_index ASC, created_at ASC`, stockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Image, 0)
	for rows.Next() {
		var img model.Image
		var rangeID sql.NullString
		var created, updated string
		if err := rows.Scan(&img.ID, &img.StockID, &img.OriginalName, &img.StoredName, &img.StoredPath, &img.ContentType, &img.OrderIndex, &rangeID, &created, &updated); err != nil {
			return nil, err
		}
		if rangeID.Valid {
			v := rangeID.String
			img.RangeID = &v
		}
		img.CreatedAt, _ = parseTime(created)
		img.UpdatedAt, _ = parseTime(updated)
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *Store) GetImage(id string) (model.Image, error) {
	row := s.db.QueryRow(`SELECT id,stock_id,original_name,stored_name,stored_path,content_type,order_index,range_id,created_at,updated_at FROM images WHERE id=?`, id)
	var img model.Image
	var rangeID sql.NullString
	var created, updated string
	if err := row.Scan(&img.ID, &img.StockID, &img.OriginalName, &img.StoredName, &img.StoredPath, &img.ContentType, &img.OrderIndex, &rangeID, &created, &updated); err != nil {
		return model.Image{}, err
	}
	if rangeID.Valid {
		v := rangeID.String
		img.RangeID = &v
	}
	img.CreatedAt, _ = parseTime(created)
	img.UpdatedAt, _ = parseTime(updated)
	return img, nil
}

func (s *Store) DeleteImage(id string) error {
	row := s.db.QueryRow(`SELECT stored_path FROM images WHERE id=?`, id)
	var storedPath string
	if err := row.Scan(&storedPath); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM images WHERE id=?`, id); err != nil {
		return err
	}
	_ = os.Remove(storedPath)
	return nil
}

func (s *Store) ReorderImages(stockID string, ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for idx, id := range ids {
		if _, err := tx.Exec(`UPDATE images SET order_index=?, updated_at=? WHERE id=? AND stock_id=?`, idx, now(), id, stockID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AssignImageRange(imageID, rangeID string) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE images SET range_id=?, updated_at=? WHERE id=?`, nullableString(rangeID), ts, imageID)
	return err
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullableStringPtr(v *string) any {
	if v == nil {
		return nil
	}
	return nullableString(*v)
}

func timePtrString(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.UTC().Format(time.RFC3339Nano)
}

func (s *Store) StockDetail(id string) (model.FilmStock, model.FilmModel, model.Camera, []model.FrameRange, []model.Image, error) {
	stock, err := s.GetStock(id)
	if err != nil {
		return model.FilmStock{}, model.FilmModel{}, model.Camera{}, nil, nil, err
	}
	modelRow, err := s.GetFilmModel(stock.ModelID)
	if err != nil {
		return model.FilmStock{}, model.FilmModel{}, model.Camera{}, nil, nil, err
	}
	camera, err := s.GetCamera(stock.CameraID)
	if err != nil {
		return model.FilmStock{}, model.FilmModel{}, model.Camera{}, nil, nil, err
	}
	ranges, err := s.ListRanges(id)
	if err != nil {
		return model.FilmStock{}, model.FilmModel{}, model.Camera{}, nil, nil, err
	}
	images, err := s.ListImages(id)
	if err != nil {
		return model.FilmStock{}, model.FilmModel{}, model.Camera{}, nil, nil, err
	}
	return stock, modelRow, camera, ranges, images, nil
}
