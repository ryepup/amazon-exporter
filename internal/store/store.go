package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"

	"github.com/ryepup/amazon-exporter/internal/models"
)

// Store wraps our sqlite interaction
type Store struct {
	db *sql.DB
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Save(request models.Order) (created bool, err error) {

	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// uniqify the items
	slices.Sort(request.Items)
	request.Items = slices.Compact(request.Items)

	// Save items to the database and get their IDs
	itemIDs := make([]int64, 0, len(request.Items))
	for _, item := range request.Items {
		itemID, err := s.saveItem(item, tx)
		if err != nil {
			return false, err
		}
		itemIDs = append(itemIDs, itemID)
	}

	exists, err := s.hasOrder(request.ID, tx)
	if err != nil {
		return false, err
	}
	// delete any previous records
	if exists {
		_, err = tx.Exec("DELETE FROM purchase_items WHERE purchase_id = ?", request.ID)
		if err != nil {
			return false, fmt.Errorf("existing purchase_items not deleted: %w", err)
		}
		_, err = tx.Exec("DELETE FROM purchases WHERE id = ?", request.ID)
		if err != nil {
			return false, fmt.Errorf("existing purchase not deleted: %w", err)
		}
	}

	// Save purchase information to the database
	_, err = tx.Exec(`
			INSERT INTO purchases (id, href, price, card, amount, date)
			VALUES (?, ?, ?, ?, ?, ?)
		`, request.ID, request.Href, request.Price, request.Charge.Card, request.Charge.Amount, request.Charge.Date)
	if err != nil {
		return false, fmt.Errorf("purchase not inserted: %w", err)
	}

	// Relate items to the purchase in the purchase_items table
	for _, itemID := range itemIDs {
		_, err := tx.Exec("INSERT INTO purchase_items (purchase_id, item_id) VALUES (?, ?)", request.ID, itemID)
		if err != nil {
			return false, fmt.Errorf("purchase item not inserted: %w", err)
		}
	}

	return !exists, tx.Commit()
}

func (s *Store) saveItem(item string, tx *sql.Tx) (int64, error) {
	var existingID int64
	err := tx.QueryRow("SELECT id FROM items WHERE item = ?", item).Scan(&existingID)
	switch {
	case err == nil:
		return existingID, nil
	case errors.Is(err, sql.ErrNoRows):
		result, err := tx.Exec("INSERT INTO items (item) VALUES (?)", item)
		if err != nil {
			return 0, fmt.Errorf("item not inserted: %w", err)
		}
		itemID, _ := result.LastInsertId()
		return itemID, nil
	default:
		return 0, err
	}
}

func (s *Store) hasOrder(ID string, tx *sql.Tx) (bool, error) {
	var existingID string
	err := tx.QueryRow("SELECT id FROM purchases WHERE id = ?", ID).Scan(&existingID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (s *Store) Search(ctx context.Context, query string) ([]models.Order, error) {
	log.Printf("Search(%v)", query)
	if n, err := strconv.ParseFloat(query, 32); err == nil {
		return s.loadByPriceOrAmount(ctx, n)
	} else if n, err := strconv.ParseInt(query, 10, 32); err == nil {
		return s.loadByPriceOrAmount(ctx, float64(n))
	}
	return s.loadBySearch(ctx, query)
}

func (s *Store) Load(id string) (models.Order, error) {
	// Query to fetch purchase details and associated items
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.id = ?;
    `

	// Execute the query
	rows, err := s.db.Query(query, id)
	if err != nil {
		return models.Order{}, err
	}
	defer rows.Close()
	o, err := s.rowsToOrders(rows)
	if err != nil {
		return models.Order{}, err
	}
	return o[0], nil
}

// loadByPriceOrAmount retrieves orders from the database where either the price or the amount matches the given value.
func (s *Store) loadByPriceOrAmount(ctx context.Context, value float64) ([]models.Order, error) {
	log.Printf("loadByPriceOrAmount(%v)", value)
	// Query to fetch orders based on price or amount
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.price BETWEEN (?-0.001) AND (?+0.001) 
			OR p.amount BETWEEN (?-0.001) AND (?+0.001)
    `

	// Execute the query
	rows, err := s.db.QueryContext(ctx, query, value, value, value, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.rowsToOrders(rows)
}

// loadBySearch retrieves orders from the database where the card, item, or date contains the given string.
func (s *Store) loadBySearch(ctx context.Context, search string) ([]models.Order, error) {
	log.Printf("loadBySearch(%v)", search)

	// Query to fetch orders based on card, item, or date containing the search string
	query := `
        SELECT
            p.id,
            p.href,
            p.price,
            p.card,
            p.amount,
            p.date,
            i.item
        FROM
            purchases p
            LEFT JOIN purchase_items pi ON p.id = pi.purchase_id
            LEFT JOIN items i ON pi.item_id = i.id
        WHERE
            p.card LIKE ? OR i.item LIKE ? OR p.date LIKE ?;
    `

	// Execute the query
	rows, err := s.db.QueryContext(ctx, query, "%"+search+"%", "%"+search+"%", "%"+search+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.rowsToOrders(rows)
}

func (s *Store) rowsToOrders(rows *sql.Rows) ([]models.Order, error) {
	// Map to store order details and associated items
	orderData := make(map[string]*models.Order)

	// Iterate over the rows and collect data
	for rows.Next() {
		var (
			orderID string
			href    string
			price   float64
			card    string
			amount  float64
			date    string
			item    sql.NullString
		)

		if err := rows.Scan(&orderID, &href, &price, &card, &amount, &date, &item); err != nil {
			return nil, err
		}

		// Check if the order already exists in the map
		if existingOrder, ok := orderData[orderID]; ok {
			// Add the item to the existing order's items
			if item.Valid {
				existingOrder.Items = append(existingOrder.Items, item.String)
			}
		} else {
			// Create a new order and add it to the map
			newOrder := models.Order{
				ID:    orderID,
				Href:  href,
				Items: []string{},
				Price: price,
				Charge: models.Charge{
					Card:   card,
					Amount: amount,
					Date:   date,
				},
			}

			// Add the item to the new order's items
			if item.Valid {
				newOrder.Items = append(newOrder.Items, item.String)
			}

			orderData[orderID] = &newOrder
		}
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	orders := make([]models.Order, 0, len(orderData))
	for _, order := range orderData {
		orders = append(orders, *order)
	}

	slices.SortFunc(orders, func(a, b models.Order) int {
		// sort DESC
		return b.Charge.CmpTime(a.Charge)
	})
	return orders, nil
}
