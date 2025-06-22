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
	"net/url"
	"time"

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
	RecordCategories(context.Context, map[models.TransactionID]models.TransactionUpdate) error
}

type YNAB interface {
	Unapproved(context.Context, models.BudgetID) ([]models.UnapprovedTransaction, error)
	Categories(context.Context, models.BudgetID) (map[string][]models.Category, error)
	Approve(context.Context, models.BudgetID, map[models.TransactionID]models.TransactionUpdate) error
	Budgets(ctx context.Context) ([]models.Budget, error)
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

	budgets, err := u.ynabRepo.Budgets(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var budgetID models.BudgetID

	if len(budgets) > 0 {
		budgetID = budgets[0].ID
	}

	if bID := r.URL.Query().Get("budgetID"); bID != "" {
		budgetID = models.BudgetID(bID)
	}

	if budgetID == models.BudgetID("") {
		http.Error(w, "could not find budget ID", http.StatusInternalServerError)
		return
	}

	cats, err := u.ynabRepo.Categories(r.Context(), budgetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		idToName := make(map[models.CategoryID]string)
		for _, group := range cats {
			for _, cat := range group {
				idToName[cat.ID] = cat.Name
			}
		}
		updates := make(map[models.TransactionID]models.TransactionUpdate)
		for idx, cID := range r.PostForm["categoryID"] {
			if cID == "-1" {
				continue
			}
			tID := r.PostForm["transactionID"][idx]
			updates[models.TransactionID(tID)] = models.TransactionUpdate{
				CategoryID:   models.CategoryID(cID),
				Payee:        r.PostForm["payee"][idx],
				CategoryName: idToName[models.CategoryID(cID)],
			}
		}

		if err := u.ynabRepo.Approve(r.Context(), budgetID, updates); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := u.repo.RecordCategories(r.Context(), updates); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.URL.RawQuery = url.Values{"budgetID": []string{budgetID.String()}}.Encode()

		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}

	trans, err := u.ynabRepo.Unapproved(r.Context(), budgetID)
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
		Budgets      []models.Budget
		BudgetID     models.BudgetID
	}{
		Categories:   cats,
		Transactions: make([]unapproved, 0, len(trans)),
		Budgets:      budgets,
		BudgetID:     budgetID,
	}
	for _, ut := range trans {
		ut := ut
		orders, err := u.repo.Search(r.Context(), fmt.Sprintf("%.2f", math.Abs(ut.Amount)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		u := unapproved{
			UnapprovedTransaction: ut,
		}
		for _, o := range orders {
			t, err := o.Charge.Time()
			if err != nil {
				log.Printf("ignoring order %s, bad date %s", o.ID, o.Charge.Date)
				continue
			}
			diff := ut.Date.Sub(t).Abs()
			if diff < 72*time.Hour {
				u.Orders = append(u.Orders, o)
			}
		}

		templateData.Transactions = append(templateData.Transactions, u)
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
