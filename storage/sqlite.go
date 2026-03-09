package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func OpenSQLite(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS seens (
	chat_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	count INTEGER NOT NULL DEFAULT 0,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (chat_id, user_id)
);
`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create seens table: %w", err)
	}

	return db, nil
}
