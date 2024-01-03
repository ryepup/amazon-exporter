package main

import (
	"database/sql"
	"encoding/json"
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

type PutRequest struct {
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

	// Handle PUT requests
	http.HandleFunc("/purchase", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request PutRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Check if the purchase with the given ID already exists
		var existingID string
		err = db.QueryRow("SELECT id FROM purchases WHERE id = ?", request.ID).Scan(&existingID)
		if err == nil {
			// Purchase with the same ID already exists, return 409 Conflict
			http.Error(w, "Conflict: Purchase with the same ID already exists", http.StatusConflict)
			return
		} else if err != sql.ErrNoRows {
			// Other database error
			log.Println("Error checking existing purchase:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Save items to the database and get their IDs
		itemIDs := make([]int64, 0, len(request.Items))
		for _, item := range request.Items {
			result, err := db.Exec("INSERT INTO items (item) VALUES (?)", item)
			if err != nil {
				log.Println("Error inserting item:", err)
				continue
			}
			itemID, _ := result.LastInsertId()
			itemIDs = append(itemIDs, itemID)
		}

		// Save purchase information to the database
		_, err = db.Exec(`
			INSERT INTO purchases (id, href, price, card, amount, date)
			VALUES (?, ?, ?, ?, ?, ?)
		`, request.ID, request.Href, request.Price, request.Charge.Card, request.Charge.Amount, request.Charge.Date)
		if err != nil {
			log.Println("Error inserting purchase:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Relate items to the purchase in the purchase_items table
		for _, itemID := range itemIDs {
			_, err := db.Exec("INSERT INTO purchase_items (purchase_id, item_id) VALUES (?, ?)", request.ID, itemID)
			if err != nil {
				log.Println("Error relating item to purchase:", err)
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	// Start the server
	addr := fmt.Sprintf(":%d", *portFlag)
	log.Printf("Server is listening on %s...", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
