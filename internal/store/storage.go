package store

import "database/sql"

type Storage struct {
	DB      *sql.DB
	Targets TargetStorage
	Checks  CheckStorage
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := openDatabase(dbPath)
	if err != nil {
		return nil, err
	}
	targetStorage := newSQLiteTargetStore(db)
	checkStorage := newSQLiteCheckStore(db)

	return &Storage{
		DB:      db,
		Targets: targetStorage,
		Checks:  checkStorage,
	}, nil
}

func (s *Storage) Close() error {
	return s.DB.Close()
}
