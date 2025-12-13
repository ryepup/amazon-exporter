package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ryepup/amazon-exporter/internal/api"
	"github.com/ryepup/amazon-exporter/internal/fixme"
	"github.com/ryepup/amazon-exporter/internal/store"
	"github.com/ryepup/amazon-exporter/internal/ui"
	"github.com/ryepup/amazon-exporter/internal/ynab"
	_ "modernc.org/sqlite"
)

var (
	portFlag   = flag.Int("port", 8080, "Port for the HTTP server")
	dbFileFlag = flag.String("dbfile", "example.db", "SQLite database file")
	ynabToken  = flag.String("ynab-token", os.Getenv("YNAB_TOKEN"), "YNAB access token, can specify with YNAB_TOKEN")
	ynabServer = flag.String("ynab-server", "https://api.ynab.com/v1/", "YNAB api server")
	uiPath     = flag.String("ui-path", "", "Path to UI directory for dynamic template reloading")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	// Parse command-line flags
	flag.Parse()

	// Default to serve command if no subcommand provided
	command := "serve"
	args := flag.Args()
	if len(args) > 0 {
		command = args[0]
	}

	// Initialize the database
	repo, err := store.Open(*dbFileFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	ynabRepo, err := ynab.New(ynab.Config{
		Token:  *ynabToken,
		Server: *ynabServer,
	})
	if err != nil {
		log.Fatal(err)
	}
	switch command {
	case "serve":
		serve(ctx, repo, ynabRepo)
	case "fixme":
		if err := fixme.Run(ctx, repo, ynabRepo); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Available commands: serve, fixme\n")
		os.Exit(1)
	}
}

func serve(ctx context.Context, repo *store.Store, ynabRepo *ynab.YNAB) {
	u, err := ui.New(repo, ynabRepo, ui.WithUIPath(*uiPath))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.New(repo)))
	mux.Handle("/", u)

	// Start the server
	addr := fmt.Sprintf(":%d", *portFlag)
	s := http.Server{Addr: addr, Handler: withLog(mux)}
	context.AfterFunc(ctx, func() {
		cctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		s.Shutdown(cctx)
	})
	log.Printf("Server is listening on %s...", addr)
	log.Fatal(s.ListenAndServe())
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("http %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
