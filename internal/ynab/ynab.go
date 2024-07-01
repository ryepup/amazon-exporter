package ynab

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/ryepup/amazon-exporter/internal/models"
)

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml https://api.ynab.com/papi/open_api_spec.yaml

func WithAuthorization(token string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		log.Printf("ynab %s %s", req.Method, req.URL)
		return nil
	})
}

type Config struct {
	Token, Server, BudgetID string
}

type YNAB struct {
	client     *ClientWithResponses
	budgetID   string
	categories map[string][]models.Category // cache the categories
}

func New(cfg Config) (*YNAB, error) {
	c, err := NewClientWithResponses(cfg.Server, WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", cfg.Token))
		log.Printf("ynab %s %s", req.Method, req.URL)
		return nil
	}))
	if err != nil {
		return nil, err
	}

	return &YNAB{
		client:   c,
		budgetID: cfg.BudgetID,
	}, nil
}

func (y *YNAB) Unapproved(ctx context.Context) (ret []models.UnapprovedTransaction, err error) {
	res, err := y.client.GetTransactionsWithResponse(ctx, y.budgetID, &GetTransactionsParams{
		Type: ptr(GetTransactionsParamsTypeUnapproved),
	})
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("could not get transactions: %d", res.StatusCode())
	}

	for _, td := range res.JSON200.Data.Transactions {
		ret = append(ret, models.UnapprovedTransaction{
			ID:     models.TransactionID(td.Id),
			Amount: float64(td.Amount) / 1000,
			Date:   td.Date.String(),
			Payee:  first(td.ImportPayeeName, td.ImportPayeeNameOriginal, td.PayeeName),
		})
	}
	return ret, nil
}

func (y *YNAB) Categories(ctx context.Context) (map[string][]models.Category, error) {
	if y.categories != nil {
		return y.categories, nil
	}
	res, err := y.client.GetCategoriesWithResponse(ctx, y.budgetID, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("could not get categories: %d", res.StatusCode())
	}

	y.categories = make(map[string][]models.Category)
	for _, cgwc := range res.JSON200.Data.CategoryGroups {
		if cgwc.Deleted || cgwc.Hidden {
			continue
		}
		items := make([]models.Category, 0, len(cgwc.Categories))
		for _, c := range cgwc.Categories {
			if c.Hidden || c.Deleted {
				continue
			}

			items = append(items, models.Category{
				ID:   models.CategoryID(c.Id.String()),
				Name: c.Name,
			})
		}
		if len(items) > 0 {
			y.categories[cgwc.Name] = items
		}
	}
	return y.categories, nil
}

func (y *YNAB) Approve(ctx context.Context, items map[models.TransactionID]models.CategoryID) error {
	if len(items) == 0 {
		return nil
	}
	updates := UpdateTransactionsJSONRequestBody{}
	for ti, ci := range items {
		ci, err := uuid.Parse(ci.String())
		if err != nil {
			return err
		}
		updates.Transactions = append(updates.Transactions, SaveTransactionWithIdOrImportId{
			Id:         ptr(ti.String()),
			Approved:   ptr(true),
			CategoryId: ptr(ci),
			PayeeName:  ptr("Amazon"),
		})
	}

	res, err := y.client.UpdateTransactionsWithResponse(ctx, y.budgetID, updates)
	if err != nil {
		return err
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("could not get update: %d", res.StatusCode())
	}

	return nil
}

func ptr[T any](val T) *T { return &val }

func first[T any](opts ...*T) (ret T) {
	for _, v := range opts {
		if v != nil {
			return *v
		}
	}
	return ret
}
