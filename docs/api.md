# Rolly API

Base path: `/api/v1`

Content type:

- JSON for normal CRUD endpoints
- `multipart/form-data` for image upload

No auth is enabled in v1.

## Health

### `GET /healthz`

Returns:

```json
{"status":"ok"}
```

## Film models

### `GET /api/v1/film-models`

List film models.

### `POST /api/v1/film-models`

Create a film model.

Example:

```json
{
  "id": "fp4",
  "name": "Kodak Portra 400",
  "iso": 400,
  "size": "135",
  "nominal_photo_count": 36,
  "supported_processing": ["C-41"]
}
```

Fields:

- `id` optional
- `name` required
- `iso` required
- `size` required
- `nominal_photo_count` optional
- `supported_processing` optional string array

### `GET /api/v1/film-models/{id}`

### `PATCH /api/v1/film-models/{id}`

### `PUT /api/v1/film-models/{id}`

### `DELETE /api/v1/film-models/{id}`

Update payload is the same shape as create.

## Cameras

### `GET /api/v1/cameras`

### `POST /api/v1/cameras`

Example:

```json
{
  "id": "cam-1",
  "name": "Konica C35 EF3",
  "maker": "Konica",
  "model": "Konica C35 EF3",
  "serial_number": "3076451",
  "metering_mode": "Spot metering",
  "focal_length": "35",
  "focal_length_35mm": "35"
}
```

Notes:

- `focal_length` and `focal_length_35mm` accept numbers or numeric strings
- `id` optional

### `GET /api/v1/cameras/{id}`

### `PATCH /api/v1/cameras/{id}`

### `PUT /api/v1/cameras/{id}`

### `DELETE /api/v1/cameras/{id}`

## Film stocks

### `GET /api/v1/film-stocks`

### `POST /api/v1/film-stocks`

Example:

```json
{
  "id": "stock-1",
  "model_id": "fp4",
  "camera_id": "cam-1",
  "expiry_year": 2027,
  "expiry_month": 6,
  "emulsion_number": "A1234",
  "chosen_processing": "C-41",
  "scanner_model": "Epson V600",
  "comment": "Summer roll for later export"
}
```

Fields:

- `model_id` required
- `camera_id` required
- `expiry_year` required
- `expiry_month` required
- `emulsion_number` required
- `chosen_processing` required
- `scanner_model` required
- `comment` optional free-form text
- `id` optional

### `GET /api/v1/film-stocks/{id}`

Returns stock detail with:

- `stock`
- `model`
- `camera`
- `ranges`
- `images`

### `DELETE /api/v1/film-stocks/{id}`

## Ranges

### `GET /api/v1/film-stocks/{stockId}/ranges`

### `POST /api/v1/film-stocks/{stockId}/ranges`

Example:

```json
{
  "id": "range-1",
  "start_frame": 0,
  "end_frame": 18,
  "shot_from": "2026-05-10T12:00:00Z",
  "shot_to": "2026-05-10T13:00:00Z",
  "location": "park",
  "weather": "cloudy",
  "notes": "walk in the park"
}
```

Rules:

- ranges must not overlap within a stock
- `start_frame` must be `>= 0`
- `end_frame` must be `>= start_frame`
- `shot_from` and `shot_to` are optional RFC3339 timestamps

### `GET /api/v1/film-stocks/{stockId}/ranges/{rangeId}`

### `PATCH /api/v1/film-stocks/{stockId}/ranges/{rangeId}`

### `PUT /api/v1/film-stocks/{stockId}/ranges/{rangeId}`

### `DELETE /api/v1/film-stocks/{stockId}/ranges/{rangeId}`

## Images

### `GET /api/v1/film-stocks/{stockId}/images`

### `POST /api/v1/film-stocks/{stockId}/images`

Upload one or more files with field name `files`.

Example form-data:

- `files`: `scan1.jpg`
- `files`: `scan2.tiff`

Behavior:

- files are stored on disk
- each image gets a stock-wide order index
- mixed extensions can be uploaded together

### `PATCH /api/v1/film-stocks/{stockId}/images/reorder`

Reorder images by ID.

Example:

```json
{
  "image_ids": ["img-2", "img-1", "img-3"]
}
```

### `PATCH /api/v1/images/{imageId}/range`

Assign one image to one range.

Example:

```json
{
  "range_id": "range-1"
}
```

## Export

### `POST /api/v1/film-stocks/{stockId}/exports`

Exports the stock to the configured export directory.

Returns:

```json
{
  "stock_id": "stock-1",
  "output_dir": "./exports/stock-1",
  "manifest": "./exports/stock-1/manifest.json",
  "files": [
    {
      "image_id": "img-1",
      "source_path": "./uploads/stock-1/file1.jpg",
      "output_path": "./exports/stock-1/stock-1-0001-0001.jpg",
      "output_name": "stock-1-0001-0001.jpg",
      "frame_number": 0
    }
  ]
}
```

Export rules:

- JPEG stays JPEG
- TIFF stays TIFF
- metadata is rebuilt from scratch
- comment metadata is written to `UserComment`
- `Date Taken` is derived from assigned range timestamps, one second per image
- when the derived time passes `shot_to`, it stops increasing

## Errors

Errors are returned as:

```json
{"error":"message"}
```
