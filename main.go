package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/ryepup/amazon-exporter/internal/models"
	"github.com/ryepup/amazon-exporter/internal/store"
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

func main() {
	// Parse command-line flags
	flag.Parse()

	// Initialize the database
	repo, err := store.Open(*dbFileFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

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

		var request models.Order
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
		if err := tmpl.ExecuteTemplate(w, "results.html", struct {
			Orders []models.Order
			Q      string
		}{orders, q}); err != nil {
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

func addCors(h http.Header) {
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	h.Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
}
