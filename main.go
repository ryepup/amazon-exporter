package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"

	_ "modernc.org/sqlite"
)

var (
	portFlag   = flag.Int("port", 8080, "Port for the HTTP server")
	dbFileFlag = flag.String("dbfile", "example.db", "SQLite database file")
	//go:embed static
	static embed.FS
	//go:embed templates/*
	templateFS embed.FS
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

	staticFS, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}
	staticServer := http.FileServer(http.FS(staticFS))

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	// Handle PUT requests
	http.HandleFunc("/api/purchases", func(w http.ResponseWriter, r *http.Request) {
		addCors(w.Header())
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
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

	http.HandleFunc("/purchases", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		orders, err := repo.Search(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "results.html", struct{ Orders []*Order }{orders}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		default:
			staticServer.ServeHTTP(w, r)
		}
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

func (r *repository) Search(query string) ([]*Order, error) {
	log.Printf("Search(%v)", query)
	if n, err := strconv.ParseFloat(query, 32); err == nil {
		return r.loadByPriceOrAmount(n)
	} else if n, err := strconv.ParseInt(query, 10, 32); err == nil {
		return r.loadByPriceOrAmount(float64(n))
	}
	return r.loadBySearch(query)
}

func (r *repository) Load(id string) (*Order, error) {
	// Query to fetch purchase details and associated items
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.id = ?;
    `

	// Execute the query
	rows, err := r.db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store purchase details and associated items
	purchaseData := make(map[string]interface{})

	// Iterate over the rows and collect data
	for rows.Next() {
		var (
			purchaseID string
			href       string
			price      float64
			card       string
			amount     float64
			date       string
			item       sql.NullString
		)

		if err := rows.Scan(&purchaseID, &href, &price, &card, &amount, &date, &item); err != nil {
			return nil, err
		}

		// Add data to the map
		if _, ok := purchaseData[purchaseID]; !ok {
			purchaseData[purchaseID] = map[string]interface{}{
				"id":     purchaseID,
				"href":   href,
				"price":  price,
				"card":   card,
				"amount": amount,
				"date":   date,
				"items":  []string{},
			}
		}

		if item.Valid {
			purchaseData[purchaseID].(map[string]interface{})["items"] = append(
				purchaseData[purchaseID].(map[string]interface{})["items"].([]string),
				item.String,
			)
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return the result
	if data, ok := purchaseData[id]; ok {
		return &Order{
			ID:    data.(map[string]interface{})["id"].(string),
			Href:  data.(map[string]interface{})["href"].(string),
			Items: data.(map[string]interface{})["items"].([]string),
			Price: data.(map[string]interface{})["price"].(float64),
			Charge: struct {
				Card   string  `json:"card"`
				Amount float64 `json:"amount"`
				Date   string  `json:"date"`
			}{
				Card:   data.(map[string]interface{})["card"].(string),
				Amount: data.(map[string]interface{})["amount"].(float64),
				Date:   data.(map[string]interface{})["date"].(string),
			},
		}, nil
	}

	return nil, errors.New("record not found")
}

// loadByPriceOrAmount retrieves orders from the database where either the price or the amount matches the given value.
func (r *repository) loadByPriceOrAmount(value float64) ([]*Order, error) {
	log.Printf("loadByPriceOrAmount(%v)", value)
	// Query to fetch orders based on price or amount
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.price BETWEEN (?-0.001) AND (?+0.001) 
			OR p.amount BETWEEN (?-0.001) AND (?+0.001) 
    `

	// Execute the query
	rows, err := r.db.Query(query, value, value, value, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store order details and associated items
	orderData := make(map[string]*Order)
	orders := make([]*Order, 0)

	// Iterate over the rows and collect data
	for rows.Next() {
		var (
			orderID string
			href    string
			price   float64
			card    string
			amount  float64
			date    string
			item    sql.NullString
		)

		if err := rows.Scan(&orderID, &href, &price, &card, &amount, &date, &item); err != nil {
			return nil, err
		}

		// Check if the order already exists in the map
		if existingOrder, ok := orderData[orderID]; ok {
			// Add the item to the existing order's items
			if item.Valid {
				existingOrder.Items = append(existingOrder.Items, item.String)
			}
		} else {
			// Create a new order and add it to the map
			newOrder := &Order{
				ID:    orderID,
				Href:  href,
				Items: []string{},
				Price: price,
				Charge: struct {
					Card   string  `json:"card"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}{
					Card:   card,
					Amount: amount,
					Date:   date,
				},
			}

			// Add the item to the new order's items
			if item.Valid {
				newOrder.Items = append(newOrder.Items, item.String)
			}

			orderData[orderID] = newOrder
			orders = append(orders, newOrder)
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// loadBySearch retrieves orders from the database where the card, item, or date contains the given string.
func (r *repository) loadBySearch(search string) ([]*Order, error) {
	log.Printf("loadBySearch(%v)", search)

	// Query to fetch orders based on card, item, or date containing the search string
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.card LIKE ? OR i.item LIKE ? OR p.date LIKE ?;
    `

	// Execute the query
	rows, err := r.db.Query(query, "%"+search+"%", "%"+search+"%", "%"+search+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store order details and associated items
	orderData := make(map[string]*Order)
	orders := make([]*Order, 0)

	// Iterate over the rows and collect data
	for rows.Next() {
		var (
			orderID string
			href    string
			price   float64
			card    string
			amount  float64
			date    string
			item    sql.NullString
		)

		if err := rows.Scan(&orderID, &href, &price, &card, &amount, &date, &item); err != nil {
			return nil, err
		}

		// Check if the order already exists in the map
		if existingOrder, ok := orderData[orderID]; ok {
			// Add the item to the existing order's items
			if item.Valid {
				existingOrder.Items = append(existingOrder.Items, item.String)
			}
		} else {
			// Create a new order and add it to the map
			newOrder := &Order{
				ID:    orderID,
				Href:  href,
				Items: []string{},
				Price: price,
				Charge: struct {
					Card   string  `json:"card"`
					Amount float64 `json:"amount"`
					Date   string  `json:"date"`
				}{
					Card:   card,
					Amount: amount,
					Date:   date,
				},
			}

			// Add the item to the new order's items
			if item.Valid {
				newOrder.Items = append(newOrder.Items, item.String)
			}

			orderData[orderID] = newOrder
			orders = append(orders, newOrder)
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func addCors(h http.Header) {
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	h.Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
}
