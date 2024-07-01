package ui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"

	"github.com/ryepup/amazon-exporter/internal/models"
)

var (
	//go:embed static
	static embed.FS
	//go:embed templates/*
	templateFS embed.FS
)

type Repo interface {
	Search(context.Context, string) ([]models.Order, error)
}

type YNAB interface {
	Unapproved(context.Context) ([]models.UnapprovedTransaction, error)
	Categories(context.Context) (map[string][]models.Category, error)
	Approve(context.Context, map[models.TransactionID]models.CategoryID) error
}

type UI struct {
	staticServer http.Handler
	templates    template.Template
	repo         Repo
	ynabRepo     YNAB
}

func New(repo Repo, y YNAB) (*UI, error) {
	staticFS, err := fs.Sub(static, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to make static subtree: %w", err)
	}

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	return &UI{
		staticServer: http.FileServer(http.FS(staticFS)),
		templates:    *tmpl,
		repo:         repo,
		ynabRepo:     y,
	}, nil
}

func (u *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/purchases":
		u.results(w, r)
	case "/":
		u.index(w, r)
	case "/ynab":
		u.ynab(w, r)
	default:
		u.staticServer.ServeHTTP(w, r)
	}
}

func (u *UI) index(w http.ResponseWriter, r *http.Request) {
	templateData := struct {
		Orders []models.Order
		Q      string
	}{}

	q := r.URL.Query().Get("q")
	if q != "" {
		orders, err := u.repo.Search(r.Context(), q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		templateData.Orders = orders
		templateData.Q = q
	}
	u.renderPage(w, "index.html", templateData)

}

func (u *UI) results(w http.ResponseWriter, r *http.Request) {
	templateData := struct {
		Orders []models.Order
		Q      string
	}{}

	q := r.URL.Query().Get("q")
	if q != "" {
		orders, err := u.repo.Search(r.Context(), q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		templateData.Orders = orders
		templateData.Q = q
	}
	if err := u.templates.ExecuteTemplate(w, "results.html", templateData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (u *UI) ynab(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		updates := make(map[models.TransactionID]models.CategoryID)
		for idx, cID := range r.PostForm["categoryID"] {
			if cID == "-1" {
				continue
			}
			tID := r.PostForm["transactionID"][idx]
			updates[models.TransactionID(tID)] = models.CategoryID(cID)
		}

		if err := u.ynabRepo.Approve(r.Context(), updates); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, r.URL.Path, http.StatusFound)
		return
	}

	cats, err := u.ynabRepo.Categories(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	trans, err := u.ynabRepo.Unapproved(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type unapproved struct {
		models.UnapprovedTransaction
		Orders []models.Order
	}

	templateData := struct {
		Transactions []unapproved
		Categories   map[string][]models.Category
	}{
		Categories:   cats,
		Transactions: make([]unapproved, 0, len(trans)),
	}
	for _, ut := range trans {
		ut := ut
		orders, err := u.repo.Search(r.Context(), fmt.Sprintf("%.2f", math.Abs(ut.Amount)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// TODO: filter orders to be near to ut.Date
		templateData.Transactions = append(templateData.Transactions, unapproved{
			UnapprovedTransaction: ut,
			Orders:                orders,
		})
	}
	u.renderPage(w, "ynab.html", templateData)
}

func (u *UI) renderPage(w http.ResponseWriter, page string, templateData any) {
	p, err := u.templates.Clone()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p.AddParseTree("content", u.templates.Lookup(page).Tree)
	if err := p.ExecuteTemplate(w, "base.html", templateData); err != nil {
		log.Printf("render failure: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}
