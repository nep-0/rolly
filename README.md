# Rolly

Rolly is a Go film stock management system with a REST API and a simple browser frontend.

## What it does

- Manage film models, cameras, and film stocks
- Attach frame ranges and scan notes to each stock
- Store free-form comments on each film stock
- Upload scanner images in batches
- Reorder uploaded images
- Assign each image to one range
- Export JPEG or TIFF images with fresh EXIF metadata

## Run

```powershell
go run ./cmd/rolly
```

Defaults:

- API: `127.0.0.1:8080`
- SQLite DB: `./rolly.db`
- Uploads: `./uploads`
- Exports: `./exports`
- Frontend: `./frontend`

You can override them:

```powershell
go run ./cmd/rolly -addr 127.0.0.1:8080 -db .\rolly.db -uploads .\uploads -exports .\exports -frontend .\frontend
```

Open `http://127.0.0.1:8080/`.

## Storage

- SQLite via `modernc.org/sqlite`
- Uploaded source files are stored on disk
- Exported files are written to the export directory

## Metadata

Rolly writes EXIF from scratch on export.

- JPEG export uses `github.com/dsoprea/go-jpeg-image-structure/v2`
- TIFF export uses `github.com/dsoprea/go-tiff-image-structure/v2`
- Human-readable range metadata goes into `UserComment`
- Date Taken is derived from range info when available

## Frontend

The `frontend/` folder is served directly by the Go server. It is a lightweight test UI, not a separate build.

## API docs

See [docs/api.md](docs/api.md).
