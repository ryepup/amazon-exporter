// handlers for the rest API
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/ryepup/amazon-exporter/internal/models"
)

type Repo interface {
	Save(models.Order) (bool, error)
}

type purchases struct {
	repo Repo
}

func (p *purchases) put(r *http.Request) (int, error) {
	var request models.Order
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return 0, err
	}
	// sanity check
	if !strings.HasSuffix(r.URL.Path, request.ID) {
		return http.StatusBadRequest, nil
	}

	created, err := p.repo.Save(request)
	if err != nil {
		return 0, err
	}
	if created {
		return http.StatusCreated, nil
	}
	return http.StatusOK, nil
}

func (p *purchases) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		code, err := p.put(r)
		if err != nil {
			log.Println("Error posting purchase:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(code)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func New(repo Repo) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/purchases/", &purchases{repo})

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Access-Control-Allow-Origin", "*")
		h.Add("Access-Control-Allow-Methods", "POST, PUT, GET, OPTIONS")
		h.Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
