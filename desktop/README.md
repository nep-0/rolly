# Rolly Desktop

This folder contains the Electron wrapper for the Rolly Go backend.

## Flow

1. Electron starts.
2. Electron launches the Go binary.
3. Electron picks a free `127.0.0.1` port and launches the backend there.
4. Electron opens that local URL in a window.

## Backend location

- Default packaged path: `process.resourcesPath/backend/rolly.exe`
- Override for local testing: `ROLLY_BACKEND_PATH`

## Frontend location

- Default packaged path: `process.resourcesPath/frontend`
- Override for local testing: `ROLLY_FRONTEND_DIR`

## Runtime data

The app stores SQLite, uploads, and exports under Electron user data:

- database: `rolly-data/rolly.db`
- uploads: `rolly-data/uploads`
- exports: `rolly-data/exports`

Electron also launches the backend with its working directory set to `rolly-data` and passes the same paths through both CLI flags and `ROLLY_*` environment variables. This avoids accidentally using `./rolly.db` from the project folder.

## Change app data directory

Use the application menu:

- `Rolly` -> `Set App Data Directory...`

The backend restarts after the directory changes. The selected path is saved in Electron user data for later launches.

## Local test

```powershell
cd desktop
npm install
npm run start
```
