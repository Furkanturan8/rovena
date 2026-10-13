package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// MaxAddressesPerUser keeps one account from storing an unbounded address book.
const MaxAddressesPerUser = 10

type Address struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Title      string // "Home", "Office", ...
	FullName   string
	Phone      string
	Line1      string
	Line2      string
	District   string
	City       string
	PostalCode string
	Country    string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrAddressNotFound  = errors.New("address not found")
	ErrTooManyAddresses = errors.New("too many saved addresses")
	ErrWrongPassword    = errors.New("current password is incorrect")
	ErrSamePassword     = errors.New("new password must differ from the current one")
	ErrEmailUnchanged   = errors.New("email is unchanged")
)
