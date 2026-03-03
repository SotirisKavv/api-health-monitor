package store

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

func migrate(db *sql.DB) error {
	if schemaSQL == "" {
		return fmt.Errorf("migration schema is empty")
	}
	_, err := db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}
	return nil
}
