package store

import "database/sql"

func initDatabase(path string) (*sql.DB, error) {
	// Open SQLite database
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Create items table if not exists with a UNIQUE constraint
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item TEXT UNIQUE
		)
	`)
	if err != nil {
		return nil, err
	}

	// Create purchases table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS purchases (
			id TEXT PRIMARY KEY,
			href TEXT,
			price REAL,
			card TEXT,
			amount REAL,
			date TEXT
		)
	`)
	if err != nil {
		return nil, err
	}

	// Create purchase_items table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS purchase_items (
			purchase_id TEXT,
			item_id INTEGER,
			FOREIGN KEY(purchase_id) REFERENCES purchases(id),
			FOREIGN KEY(item_id) REFERENCES items(id),
			PRIMARY KEY (purchase_id, item_id)
		)
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		return nil, err
	}

	// prevent "database is locked (5) (SQLITE_BUSY)" errors on concurrent
	// access
	db.SetMaxOpenConns(1)

	return db, nil
}

func Open(path string) (*Store, error) {
	db, err := initDatabase(path)
	if err != nil {
		return nil, err
	}
	return &Store{db}, nil
}
