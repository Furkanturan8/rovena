package application

import (
	"context"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
)

type AddressService struct {
	addresses repository.AddressRepository
	now       func() time.Time
}

func NewAddressService(addresses repository.AddressRepository) *AddressService {
	return &AddressService{addresses: addresses, now: time.Now}
}

type AddressInput struct {
	Title      string
	FullName   string
	Phone      string
	Line1      string
	Line2      string
	District   string
	City       string
	PostalCode string
	IsDefault  bool
}

func (s *AddressService) List(ctx context.Context, userID uuid.UUID) ([]domain.Address, error) {
	return s.addresses.List(ctx, userID)
}

func (s *AddressService) Create(ctx context.Context, userID uuid.UUID, in AddressInput) (*domain.Address, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.now()
	a := &domain.Address{
		ID: id, UserID: userID, Country: "TR", IsDefault: in.IsDefault, CreatedAt: now, UpdatedAt: now,
	}
	applyAddress(a, in)
	if err := s.addresses.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// AddressPatch carries only the fields the caller wants to change.
type AddressPatch struct {
	Title      *string
	FullName   *string
	Phone      *string
	Line1      *string
	Line2      *string
	District   *string
	City       *string
	PostalCode *string
	IsDefault  *bool
}

func (s *AddressService) Update(ctx context.Context, userID, id uuid.UUID, p AddressPatch) (*domain.Address, error) {
	a, err := s.addresses.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&a.Title, p.Title)
	set(&a.FullName, p.FullName)
	set(&a.Phone, p.Phone)
	set(&a.Line1, p.Line1)
	set(&a.Line2, p.Line2)
	set(&a.District, p.District)
	set(&a.City, p.City)
	set(&a.PostalCode, p.PostalCode)
	if err := s.addresses.Update(ctx, a, p.IsDefault != nil && *p.IsDefault); err != nil {
		return nil, err
	}
	return s.addresses.Get(ctx, userID, id)
}

func (s *AddressService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.addresses.Delete(ctx, userID, id)
}

func (s *AddressService) SetDefault(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	return s.addresses.SetDefault(ctx, userID, id)
}

func applyAddress(a *domain.Address, in AddressInput) {
	a.Title = strings.TrimSpace(in.Title)
	a.FullName = strings.TrimSpace(in.FullName)
	a.Phone = strings.TrimSpace(in.Phone)
	a.Line1 = strings.TrimSpace(in.Line1)
	a.Line2 = strings.TrimSpace(in.Line2)
	a.District = strings.TrimSpace(in.District)
	a.City = strings.TrimSpace(in.City)
	a.PostalCode = strings.TrimSpace(in.PostalCode)
}
