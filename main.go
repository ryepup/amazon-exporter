package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ryepup/amazon-exporter/internal/api"
	"github.com/ryepup/amazon-exporter/internal/store"
	"github.com/ryepup/amazon-exporter/internal/ui"
	"github.com/ryepup/amazon-exporter/internal/ynab"
	_ "modernc.org/sqlite"
)

var (
	portFlag   = flag.Int("port", 8080, "Port for the HTTP server")
	dbFileFlag = flag.String("dbfile", "example.db", "SQLite database file")
	ynabToken  = flag.String("ynab-token", os.Getenv("YNAB_TOKEN"), "YNAB access token, can specify with YNAB_TOKEN")
	ynabBudget = flag.String("ynab-budget", "", "YNAB budget ID") // TODO: make this selectable on the UI
	ynabServer = flag.String("ynab-server", "https://api.ynab.com/v1/", "YNAB api server")
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

	ynabRepo, err := ynab.New(ynab.Config{
		Token:    *ynabToken,
		Server:   *ynabServer,
		BudgetID: *ynabBudget,
	})
	if err != nil {
		log.Fatal(err)
	}

	u, err := ui.New(repo, ynabRepo)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.New(repo)))
	mux.Handle("/", u)

	// Start the server
	addr := fmt.Sprintf(":%d", *portFlag)
	log.Printf("Server is listening on %s...", addr)
	log.Fatal(http.ListenAndServe(addr, withLog(mux)))
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("http %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
