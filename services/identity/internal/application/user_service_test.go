package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/google/uuid"
)

func seedUser(t *testing.T, f *fakeUsers, email string, role domain.Role) *domain.User {
	t.Helper()
	id, _ := uuid.NewV7()
	u := &domain.User{ID: id, Email: email, Role: role}
	if err := f.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestChangeRole_CannotDemoteLastAdmin(t *testing.T) {
	users := newFakeUsers()
	admin := seedUser(t, users, "admin@example.com", domain.RoleAdmin)
	svc := NewUserService(users)

	_, err := svc.ChangeRole(context.Background(), admin.ID, domain.RoleCustomer)
	if !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("err = %v, want ErrLastAdmin", err)
	}
}

func TestChangeRole_AllowsDemoteWhenAnotherAdminExists(t *testing.T) {
	users := newFakeUsers()
	a := seedUser(t, users, "a@example.com", domain.RoleAdmin)
	seedUser(t, users, "b@example.com", domain.RoleAdmin)
	svc := NewUserService(users)

	got, err := svc.ChangeRole(context.Background(), a.ID, domain.RoleCustomer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != domain.RoleCustomer {
		t.Errorf("role = %q", got.Role)
	}
}

func TestChangeRole_InvalidRole(t *testing.T) {
	users := newFakeUsers()
	u := seedUser(t, users, "u@example.com", domain.RoleCustomer)

	_, err := NewUserService(users).ChangeRole(context.Background(), u.ID, "superuser")
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("err = %v, want ErrInvalidRole", err)
	}
}
