package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

var (
	portFlag   = flag.Int("port", 8080, "Port for the HTTP server")
	dbFileFlag = flag.String("dbfile", "example.db", "SQLite database file")
)

type Order struct {
	ID     string   `json:"id"`
	Href   string   `json:"href"`
	Items  []string `json:"items"`
	Price  float64  `json:"price"`
	Charge struct {
		Card   string  `json:"card"`
		Amount float64 `json:"amount"`
		Date   string  `json:"date"`
	} `json:"charge"`
}

func initDatabase() (*sql.DB, error) {
	// Open SQLite database
	db, err := sql.Open("sqlite", *dbFileFlag)
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

func main() {
	// Parse command-line flags
	flag.Parse()

	// Initialize the database
	db, err := initDatabase()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	repo := repository{db: db}

	// Handle PUT requests
	http.HandleFunc("/purchases", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request Order
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Check if the purchase with the given ID already exists
		exists, err := repo.HasOrder(request.ID)
		if err != nil {
			log.Println("Error checking existing purchase:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if exists {
			// Purchase with the same ID already exists, return 409 Conflict
			http.Error(w, "Conflict: Purchase with the same ID already exists", http.StatusConflict)
			return
		}

		if err := repo.Save(request); err != nil {
			log.Println("Error saving:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Start the server
	addr := fmt.Sprintf(":%d", *portFlag)
	log.Printf("Server is listening on %s...", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// repository wraps our sqlite interaction
type repository struct {
	db *sql.DB
}

func (r *repository) HasOrder(ID string) (bool, error) {
	var existingID string
	err := r.db.QueryRow("SELECT id FROM purchases WHERE id = ?", ID).Scan(&existingID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (r *repository) Save(request Order) error {

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save items to the database and get their IDs
	itemIDs := make([]int64, 0, len(request.Items))
	for _, item := range request.Items {
		var existingID int64
		err := tx.QueryRow("SELECT id FROM items WHERE item = ?", item).Scan(&existingID)
		switch {
		case err == nil:
			itemIDs = append(itemIDs, existingID)
		case errors.Is(err, sql.ErrNoRows):
			result, err := tx.Exec("INSERT INTO items (item) VALUES (?)", item)
			if err != nil {
				return fmt.Errorf("item not inserted: %w", err)
			}
			itemID, _ := result.LastInsertId()
			itemIDs = append(itemIDs, itemID)
		default:
			return err
		}
	}

	// Save purchase information to the database
	_, err = tx.Exec(`
			INSERT INTO purchases (id, href, price, card, amount, date)
			VALUES (?, ?, ?, ?, ?, ?)
		`, request.ID, request.Href, request.Price, request.Charge.Card, request.Charge.Amount, request.Charge.Date)
	if err != nil {
		return fmt.Errorf("purchase not inserted: %w", err)
	}

	// Relate items to the purchase in the purchase_items table
	for _, itemID := range itemIDs {
		_, err := tx.Exec("INSERT INTO purchase_items (purchase_id, item_id) VALUES (?, ?)", request.ID, itemID)
		if err != nil {
			return fmt.Errorf("purchase item not inserted: %w", err)
		}
	}

	return tx.Commit()
}
