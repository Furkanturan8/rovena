package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/google/uuid"
)

type Inventory struct{ base }

func NewInventory(url, internalKey string, timeout time.Duration) *Inventory {
	return &Inventory{newBase("inventory", url, internalKey, timeout)}
}

var _ application.Inventory = (*Inventory)(nil)

type errorBody struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

// Reserve asks Inventory to hold the stock. Inventory answers 201 for a new
// reservation and 200 when the order already had one; both mean "held".
func (i *Inventory) Reserve(ctx context.Context, orderID uuid.UUID, lines []application.ReservationLine) (*application.Reservation, error) {
	type item struct {
		VariantID string `json:"variant_id"`
		Quantity  int    `json:"quantity"`
	}
	items := make([]item, len(lines))
	for n, l := range lines {
		items[n] = item{VariantID: l.VariantID.String(), Quantity: l.Quantity}
	}
	status, body, err := i.do(ctx, http.MethodPost, "/api/v1/internal/reservations",
		map[string]any{"order_id": orderID.String(), "items": items})
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}

	switch status {
	case http.StatusOK, http.StatusCreated:
		var env struct {
			Data struct {
				ID        uuid.UUID `json:"id"`
				ExpiresAt time.Time `json:"expires_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("inventory sent an unreadable reservation: %w", err)
		}
		if env.Data.ID == uuid.Nil {
			return nil, errors.New("inventory sent a reservation without an id")
		}
		return &application.Reservation{ID: env.Data.ID, ExpiresAt: env.Data.ExpiresAt}, nil
	case http.StatusConflict, http.StatusNotFound:
		// 409: not enough stock. 404: Inventory keeps no stock for that variant.
		// Both name the variant that blocked the reservation.
		var e errorBody
		_ = json.Unmarshal(body, &e)
		variantID, perr := uuid.Parse(e.Fields["variant_id"])
		if perr != nil {
			return nil, unexpected("inventory", status, body)
		}
		requested, _ := strconv.Atoi(e.Fields["requested"])
		available, _ := strconv.Atoi(e.Fields["available"])
		return nil, &application.StockShortage{
			VariantID: variantID, Requested: requested, Available: available, NotTracked: status == http.StatusNotFound,
		}
	}
	return nil, unexpected("inventory", status, body)
}

// Commit turns the hold into a sale. 404 and 409 both mean the reservation can no
// longer be committed (it expired, was released or is unknown).
func (i *Inventory) Commit(ctx context.Context, reservationID uuid.UUID) error {
	status, body, err := i.do(ctx, http.MethodPost, "/api/v1/internal/reservations/"+reservationID.String()+"/commit", nil)
	if err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound, http.StatusConflict:
		return application.ErrReservationGone
	}
	return unexpected("inventory", status, body)
}

// Release frees the hold. An unknown reservation counts as released.
func (i *Inventory) Release(ctx context.Context, reservationID uuid.UUID) error {
	status, body, err := i.do(ctx, http.MethodPost, "/api/v1/internal/reservations/"+reservationID.String()+"/release", nil)
	if err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	switch status {
	case http.StatusOK, http.StatusNotFound:
		return nil
	}
	return unexpected("inventory", status, body)
}
