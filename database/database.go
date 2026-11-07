package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"

	"InquiryExplorer/file"
)

func CreateSQLite() (*sql.DB, error) {
	dir, err := file.DatabaseFolderPath()
	if err != nil {
		return nil, err
	}

	if err := file.MakeDatabaseFolder(); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "app.db")

	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		filepath.ToSlash(dbPath),
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
