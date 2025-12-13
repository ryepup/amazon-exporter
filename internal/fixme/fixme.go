package fixme

import (
	"context"
	"errors"

	"github.com/ryepup/amazon-exporter/internal/store"
	"github.com/ryepup/amazon-exporter/internal/ynab"
)

// Run executes the fixme admin task
func Run(ctx context.Context, repo *store.Store, ynabRepo *ynab.YNAB) error {
	return errors.New("not implemented")
}
