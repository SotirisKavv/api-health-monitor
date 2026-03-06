package store

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openDatabase(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}
		dbPath = filepath.Join(wd, "internal/store/monitor.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Printf("Failed to ping database: %v", err)
		_ = db.Close()
		return nil, err
	}

	if err := migrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	return db, nil
}

func (s *SQLiteTargetStore) Close() error {
	return s.DB.Close()
}
