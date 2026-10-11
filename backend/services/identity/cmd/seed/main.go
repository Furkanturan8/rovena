package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/identity/internal/infrastructure/security"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository/postgres"
	"github.com/google/uuid"
)

// Creates the initial admin from ADMIN_EMAIL / ADMIN_PASSWORD. Safe to re-run.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	repo := postgres.NewUserRepository(db)
	email := application.NormalizeEmail(cfg.AdminEmail)
	if _, err := repo.GetByEmail(ctx, email); err == nil {
		fmt.Println("admin already exists:", email)
		return nil
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return err
	}

	hash, err := security.NewBcryptHasher(cfg.BcryptCost).Hash(cfg.AdminPassword)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	admin := &domain.User{ID: id, Email: email, PasswordHash: hash, FirstName: "Admin", Role: domain.RoleAdmin}
	if err := repo.Create(ctx, admin); err != nil {
		return err
	}
	fmt.Println("admin created:", email)
	return nil
}
