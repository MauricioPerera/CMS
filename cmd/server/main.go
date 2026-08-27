// Command gopress-server arranca el CMS GoPress como HTTP server listo
// para deploy (local, contenedor o Cloudflare Worker via @cloudflare/sandbox).
//
// CGO_ENABLED=1 es obligatorio (mattn/go-sqlite3 + github.com/buke/quickjs-go).
//
// Env vars:
//   PORT               Puerto a escuchar (default 8080).
//   DB_PATH            Path SQLite (default ":memory:"). Use "file:...?cache=shared"
//                      para DB en disco persistente en contenedor/Worker.
//   MIGRATIONS_DIR     Dir de migraciones (default "db/migrations").
//   CMS_REQUIRE_AUTH   =1 activa AuthRequiredEnable(true) fail-fast para writes.
//   CMS_REQUIRE_LOGGER =1 panickea NewHandler si logger no explicit (fail-fast C49).
//   CMS_BODY_LIMIT     Limite bytes para writes (default 524288 = 512KiB, 0 = unlimited).
package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gopress/internal/db"
	"gopress/internal/posts"
)

func main() {
	if err := run(); err != nil {
		slog.Default().Error("server shutdown", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	dbh, err := sql.Open("sqlite3", cfg.dbPath)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	defer dbh.Close()

	if err := db.ApplyMigrations(dbh, "file://"+filepath.ToSlash(cfg.migrationsDir)); err != nil {
		return fmt.Errorf("ApplyMigrations: %w", err)
	}

	s := posts.NewPosts(dbh, nil, nil)
	h := posts.NewHandler(s, buildOptions(cfg)...)

	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	mux.Handle("/", h)

	addr := ":" + cfg.port
	slog.Info("gopress server starting",
		"addr", addr, "db", cfg.dbPath, "auth_required", cfg.requireAuth,
		"body_limit", cfg.bodyLimit, "migrations", cfg.migrationsDir)
	return http.ListenAndServe(addr, mux)
}

type config struct {
	port          string
	dbPath        string
	migrationsDir string
	requireAuth   bool
	requireLogger bool
	bodyLimit     int64
}

func loadConfig() config {
	port := getenv("PORT", "8080")
	dbPath := getenv("DB_PATH", ":memory:")
	migrationsDir := getenv("MIGRATIONS_DIR", "db/migrations")
	requireAuth := getenv("CMS_REQUIRE_AUTH", "") == "1"
	requireLogger := getenv("CMS_REQUIRE_LOGGER", "") == "1"

	bodyLimit := int64(524288)
	if bl := getenv("CMS_BODY_LIMIT", ""); bl != "" {
		if n, err := strconv.ParseInt(bl, 10, 64); err == nil && n >= 0 {
			bodyLimit = n
		}
	}
	return config{
		port:          port,
		dbPath:        dbPath,
		migrationsDir: migrationsDir,
		requireAuth:   requireAuth,
		requireLogger: requireLogger,
		bodyLimit:     bodyLimit,
	}
}

func newLogger(cfg config) *slog.Logger {
	if cfg.requireLogger {
		return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// buildOptions construye las Option para NewHandler (fail-fast C49 auth C59/C60).
func buildOptions(cfg config) []posts.Option {
	opts := []posts.Option{
		posts.WithBodyLimit(cfg.bodyLimit),
	}
	if cfg.requireLogger {
		opts = append(opts, posts.WithLogger(slog.Default()))
	}
	if cfg.requireAuth {
		opts = append(opts, posts.AuthRequiredEnable(true))
	}
	return opts
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
