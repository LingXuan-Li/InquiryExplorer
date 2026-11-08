package database

import (
	"database/sql"
	"path/filepath"

	_ "modernc.org/sqlite"

	"InquiryExplorer/file"
)

func databasePath(name string) (string, error) {
	dir, err := file.DatabaseFolderPath()
	if err != nil {
		return "", err
	}

	if err := file.MakeDatabaseFolder(); err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}

func openDB(driver, dsn string, maxConns int) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
