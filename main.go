package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/ryepup/amazon-exporter/internal/api"
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

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.New(repo)))

	// TODO: move code into more packages:
	// - internal/ui - rendering templates for the frontend

	mux.HandleFunc("/purchases", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
	log.Fatal(http.ListenAndServe(addr, withLog(mux)))
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("http %s %s", r.Method, r.URL.Path)
	})
}
