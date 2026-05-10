package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"rolly/internal/httpapi"
	"rolly/internal/store"
)

func main() {
	var (
		addr     = flag.String("addr", getenv("ROLLY_ADDR", "127.0.0.1:8080"), "listen address")
		dbPath   = flag.String("db", getenv("ROLLY_DB", filepath.Join(".", "rolly.db")), "sqlite database path")
		uploads  = flag.String("uploads", getenv("ROLLY_UPLOAD_DIR", filepath.Join(".", "uploads")), "upload storage directory")
		exports  = flag.String("exports", getenv("ROLLY_EXPORT_DIR", filepath.Join(".", "exports")), "export output directory")
		frontend = flag.String("frontend", getenv("ROLLY_FRONTEND_DIR", filepath.Join(".", "frontend")), "frontend static directory")
	)
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*uploads, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*exports, 0o755); err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(*dbPath, *uploads, *exports)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	srv := httpapi.NewServer(st, *frontend)
	log.Printf("rolly listening on %s", *addr)
	log.Fatal(srv.ListenAndServe(*addr))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
