package fixme

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ryepup/amazon-exporter/internal/models"
	"github.com/ryepup/amazon-exporter/internal/store"
	"github.com/ryepup/amazon-exporter/internal/ynab"
)

// Run executes the fixme admin task
func Run(ctx context.Context, repo *store.Store, ynabRepo *ynab.YNAB) error {

	// add the payee column to purchase_category
	// read IDs without a payee
	// lookup tranaction in ynab
	// update row to add payee
	// rename column purchase_id to transaction_id
	// rename table purchase_category to transaction_category

	items, err := repo.OldPCs(ctx)
	if err != nil {
		return fmt.Errorf("could not read db: %w", err)
	}

	log.Printf("got %d rows to process", len(items))

	budgets, err := ynabRepo.Budgets(ctx)
	if err != nil {
		return fmt.Errorf("could not read budgets: %w", err)
	}
	log.Printf("using budget %+v", budgets[0])

	notFoundErr := errors.New("not found")
	bID := budgets[0].ID
	updates := make(map[models.TransactionID]models.TransactionUpdate, len(items))
	for _, tID := range items {

		if _, ok := updates[tID]; ok {
			return fmt.Errorf("duplicate transaction %s", tID)
		}

		t, err := backoff.Retry(ctx, func() (zero ynab.TransactionDetail, _ error) {
			r, err := ynabRepo.Client.GetTransactionByIdWithResponse(ctx, bID.String(), tID.String())
			if err != nil {
				return zero, fmt.Errorf("could not talk to YNAB: %w", err)
			}
			switch r.StatusCode() {
			case http.StatusNotFound:
				return zero, backoff.Permanent(fmt.Errorf("%s is gone: %w", tID, notFoundErr))
			case http.StatusOK:
				return r.JSON200.Data.Transaction, nil
			case http.StatusTooManyRequests:
				log.Println("too fast, waiting")
				if err := repo.RecordCategories(ctx, updates); err != nil {
					return zero, backoff.Permanent(err)
				}
				select {
				case <-ctx.Done():
					return zero, ctx.Err()
				case <-time.After(5 * time.Minute):
					return zero, fmt.Errorf("too fast!")
				}
			default:
				return zero, backoff.Permanent(fmt.Errorf("unhandled status: %d", r.StatusCode()))
			}
		},
			backoff.WithBackOff(backoff.NewExponentialBackOff()),
			backoff.WithMaxElapsedTime(2*time.Hour),
		)
		if errors.Is(err, notFoundErr) {
			log.Printf("skipping transaction: %s", err)
			continue
		}

		if err != nil {
			return fmt.Errorf("could not talk to YNAB %s: %w", tID, err)
		}

		updates[tID] = models.TransactionUpdate{
			Payee:        *t.PayeeName,
			CategoryID:   models.CategoryID(t.CategoryId.String()),
			CategoryName: *t.CategoryName,
		}
	}

	return repo.RecordCategories(ctx, updates)
}
