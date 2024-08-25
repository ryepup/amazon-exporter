package ynab

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"

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
	Token, Server string
}

type YNAB struct {
	client     *ClientWithResponses
	categories map[models.BudgetID]map[string][]models.Category // cache the categories
	budgets    []models.Budget                                  // cache the budgets
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
		client: c,
	}, nil
}

func (y *YNAB) Unapproved(ctx context.Context, budgetID models.BudgetID) (ret []models.UnapprovedTransaction, err error) {
	res, err := y.client.GetTransactionsWithResponse(ctx, budgetID.String(), &GetTransactionsParams{
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
			Date:   td.Date.Time,
			Payee:  first(td.ImportPayeeName, td.ImportPayeeNameOriginal, td.PayeeName),
		})
	}
	return ret, nil
}

func (y *YNAB) Categories(ctx context.Context, budgetID models.BudgetID) (map[string][]models.Category, error) {
	if y.categories != nil && y.categories[budgetID] != nil {
		return y.categories[budgetID], nil
	}
	res, err := y.client.GetCategoriesWithResponse(ctx, budgetID.String(), nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("could not get categories: %d", res.StatusCode())
	}

	if y.categories == nil {
		y.categories = make(map[models.BudgetID]map[string][]models.Category)
	}
	y.categories[budgetID] = make(map[string][]models.Category)

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
			y.categories[budgetID][cgwc.Name] = items
		}
	}
	return y.categories[budgetID], nil
}

func (y *YNAB) Approve(ctx context.Context, budgetID models.BudgetID, items map[models.TransactionID]models.TransactionUpdate) error {
	if len(items) == 0 {
		return nil
	}
	updates := UpdateTransactionsJSONRequestBody{}
	for ti, update := range items {
		ci, err := uuid.Parse(update.CategoryID.String())
		if err != nil {
			return err
		}
		updates.Transactions = append(updates.Transactions, SaveTransactionWithIdOrImportId{
			Id:         ptr(ti.String()),
			Approved:   ptr(true),
			CategoryId: &ci,
			PayeeName:  &update.Payee,
		})
	}

	res, err := y.client.UpdateTransactionsWithResponse(ctx, budgetID.String(), updates)
	if err != nil {
		return err
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("could not update: %d", res.StatusCode())
	}

	return nil
}

func (y *YNAB) Budgets(ctx context.Context) ([]models.Budget, error) {
	if len(y.budgets) > 0 {
		return y.budgets, nil
	}

	res, err := y.client.GetBudgetsWithResponse(ctx, &GetBudgetsParams{})
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("could not get budgets: %d", res.StatusCode())
	}

	log.Printf("bodgets: %v", res.JSON200.Data.Budgets)
	for _, b := range res.JSON200.Data.Budgets {
		if b.LastModifiedOn == nil {
			continue
		}
		y.budgets = append(y.budgets, models.Budget{
			ID:           models.BudgetID(b.Id.String()),
			Name:         b.Name,
			LastModified: *b.LastModifiedOn,
		})
	}
	slices.SortFunc(y.budgets, func(a, b models.Budget) int {
		switch {
		case a.LastModified.Equal(b.LastModified):
			return 0
		case a.LastModified.Before(b.LastModified):
			return 1
		default:
			return -1
		}
	})
	return y.budgets, nil
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
