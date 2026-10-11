package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/config"
	"github.com/Furkanturan8/rovena/services/inventory/internal/infrastructure/database"
	"github.com/Furkanturan8/rovena/services/inventory/migrations"
	"github.com/pressly/goose/v3"
)

func main() {
	action := flag.String("action", "up", "up | down | status")
	flag.Parse()

	if err := run(*action); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(action string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db := database.SQLDB(cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch action {
	case "up":
		return goose.Up(db, ".")
	case "down":
		return goose.Down(db, ".")
	case "status":
		return goose.Status(db, ".")
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}
