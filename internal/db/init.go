// Package db: inicialización de SQLite con migraciones versionadas.
// Contrato C1: db-migrations.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register driver "file" para source.Open("file://...").
)

// sourceDriver abre el source file:// usando el registry de schemes registrado
// por file.init() (source.Register("file", ...)).
func sourceDriver(sourceURL string) (source.Driver, error) {
	if !strings.HasPrefix(sourceURL, "file://") {
		return nil, fmt.Errorf("sourceURL debe usar scheme file:// (recibido: %q)", sourceURL)
	}
	src, err := source.Open(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("abrir source %q: %w", sourceURL, err)
	}
	return src, nil
}

// ApplyMigrations aplica todas las migraciones versionadas desde sourceURL
// (ej: "file://db/migrations") sobre la instancia dbh ya abierta.
func ApplyMigrations(dbh *sql.DB, sourceURL string) error {
	return runMigrations(dbh, sourceURL, func(m *migrate.Migrate) error {
		return m.Up()
	})
}

// Rollback revierte todas las migraciones hasta la versión 0.
func Rollback(dbh *sql.DB, sourceURL string) error {
	return runMigrations(dbh, sourceURL, func(m *migrate.Migrate) error {
		return m.Down()
	})
}

// runMigrations crea el Migrate ligando source + database sobre la instancia
// abierta y ejecuta action.
func runMigrations(dbh *sql.DB, sourceURL string, action func(*migrate.Migrate) error) error {
	src, err := sourceDriver(sourceURL)
	if err != nil {
		return err
	}
	dbDriver, err := sqlite.WithInstance(dbh, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("crear driver sqlite: %w", err)
	}
	m, err := migrate.NewWithInstance("file", src, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("crear migrate: %w", err)
	}
	if err := action(m); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return fmt.Errorf("ejecutar migrate: %w", err)
	}
	return nil
}

// Version retorna la versión actual de migraciones aplicadas (0 = fresh).
func Version(dbh *sql.DB, sourceURL string) (uint, error) {
	src, err := sourceDriver(sourceURL)
	if err != nil {
		return 0, err
	}
	dbDriver, err := sqlite.WithInstance(dbh, &sqlite.Config{})
	if err != nil {
		return 0, fmt.Errorf("crear driver sqlite: %w", err)
	}
	m, err := migrate.NewWithInstance("file", src, "sqlite", dbDriver)
	if err != nil {
		return 0, fmt.Errorf("crear migrate: %w", err)
	}
	v, _, err := m.Version()
	if err != nil {
		return 0, err
	}
	return v, nil
}
