// Tests del esquema baseline de migraciones.
//
// Oraculo congelado del PM (Contrato C1: db-migrations). El implementador
// NO toca este archivo: si lo modifica, el tests_sha256 del task contract
// rompe el gate FM_TESTS_FROZEN.
//
// Ejecuta: go test ./internal/db/... -run TestMigrations
// sobre una instancia SQLite en-memoria aplicada con golang-migrate.

package db

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"
)

// migrationsURL resuelve la ruta absoluta a db/migrations desde este test,
// usando runtime.Caller para ser independiente del cwd de ejecucion.
func migrationsURL() string {
	_, here, _, _ := runtime.Caller(0)
	abs := filepath.Join(filepath.Dir(here), "..", "..", "db", "migrations")
	return "file://" + filepath.ToSlash(abs)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbh, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("abrir sqlite: %v", err)
	}
	if err := ApplyMigrations(dbh, migrationsURL()); err != nil {
		t.Fatalf("aplicar migraciones: %v", err)
	}
	return dbh
}

func tableExists(t *testing.T, dbh *sql.DB, name string) {
	t.Helper()
	var found string
	err := dbh.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&found)
	if err != nil || found != name {
		t.Errorf("tabla %q no existe tras migracion (found=%q, err=%v)", name, found, err)
	}
}

func TestMigrations_CreateTables(t *testing.T) {
	dbh := openTestDB(t)
	defer dbh.Close()

	for _, tbl := range []string{"posts", "users", "sessions", "options"} {
		tableExists(t, dbh, tbl)
	}
}

func TestMigrations_PostsSchema(t *testing.T) {
	dbh := openTestDB(t)
	defer dbh.Close()

	// slug UNIQUE
	_, err := dbh.Exec("INSERT INTO posts (slug, title, content, status) VALUES ('a','t','c','draft')")
	if err != nil {
		t.Fatalf("insert post 1: %v", err)
	}
	_, err = dbh.Exec("INSERT INTO posts (slug, title, content, status) VALUES ('a','t2','c','draft')")
	if err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("esperaba error UNIQUE por slug duplicado, err=%v", err)
	}
}

func TestMigrations_UserEmailUnique(t *testing.T) {
	dbh := openTestDB(t)
	defer dbh.Close()

	_, err := dbh.Exec("INSERT INTO users (email, password_hash, display_name) VALUES ('a@b.c','h','n')")
	if err != nil {
		t.Fatalf("inserto user 1: %v", err)
	}
	_, err = dbh.Exec("INSERT INTO users (email, password_hash, display_name) VALUES ('a@b.c','h','n')")
	if err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("esperaba error UNIQUE por email duplicado, err=%v", err)
	}
}

func TestMigrations_PostStatusCheck(t *testing.T) {
	dbh := openTestDB(t)
	defer dbh.Close()

	// status invalidado por CHECK
	_, err := dbh.Exec("INSERT INTO posts (slug, title, content, status) VALUES ('s','t','c','bogus')")
	if err == nil {
		t.Fatal("esperaba error por status invalido (CHECK)")
	}
}

func TestMigrations_RollbackDropsTables(t *testing.T) {
	dbh := openTestDB(t)
	defer dbh.Close()

	// rollback version 1
	if err := Rollback(dbh, migrationsURL()); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, tbl := range []string{"posts", "users", "sessions", "options"} {
		var found string
		err := dbh.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&found)
		if err == nil {
			t.Errorf("tabla %q deberia haber sido borrada por rollback", tbl)
		}
	}
}
