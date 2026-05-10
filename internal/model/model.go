package model

import "time"

type FilmModel struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	ISO                 int       `json:"iso"`
	Size                string    `json:"size"`
	NominalPhotoCount   *int      `json:"nominal_photo_count,omitempty"`
	SupportedProcessing []string  `json:"supported_processing,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Camera struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Maker           string    `json:"maker"`
	Model           string    `json:"model"`
	SerialNumber    string    `json:"serial_number"`
	MeteringMode    string    `json:"metering_mode"`
	FocalLength     float64   `json:"focal_length"`
	FocalLength35mm float64   `json:"focal_length_35mm"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type FilmStock struct {
	ID               string    `json:"id"`
	ModelID          string    `json:"model_id"`
	CameraID         string    `json:"camera_id"`
	ExpiryYear       int       `json:"expiry_year"`
	ExpiryMonth      int       `json:"expiry_month"`
	EmulsionNumber   string    `json:"emulsion_number"`
	ChosenProcessing string    `json:"chosen_processing"`
	ScannerModel     string    `json:"scanner_model"`
	Comment          string    `json:"comment"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type FrameRange struct {
	ID         string     `json:"id"`
	StockID    string     `json:"stock_id"`
	StartFrame int        `json:"start_frame"`
	EndFrame   int        `json:"end_frame"`
	ShotFrom   *time.Time `json:"shot_from,omitempty"`
	ShotTo     *time.Time `json:"shot_to,omitempty"`
	Location   string     `json:"location,omitempty"`
	Weather    string     `json:"weather,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Image struct {
	ID           string    `json:"id"`
	StockID      string    `json:"stock_id"`
	OriginalName string    `json:"original_name"`
	StoredName   string    `json:"stored_name"`
	StoredPath   string    `json:"stored_path"`
	ContentType  string    `json:"content_type"`
	OrderIndex   int       `json:"order_index"`
	RangeID      *string   `json:"range_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ExportFile struct {
	ImageID     string `json:"image_id"`
	SourcePath  string `json:"source_path"`
	OutputPath  string `json:"output_path"`
	OutputName  string `json:"output_name"`
	FrameNumber int    `json:"frame_number"`
}

type ExportResult struct {
	StockID   string       `json:"stock_id"`
	OutputDir string       `json:"output_dir"`
	Manifest  string       `json:"manifest"`
	Files     []ExportFile `json:"files"`
}
