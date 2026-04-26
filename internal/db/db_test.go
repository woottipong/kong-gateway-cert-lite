package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndRunsMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "app.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	for _, table := range []string{
		"schema_migrations",
		"certificates",
		"kong_targets",
		"certificate_kong_targets",
		"jobs",
	} {
		if !tableExists(t, database, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}

func TestRunMigrationsIsSafeToRerun(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", "001_initial.sql").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected migration recorded once, got %d", count)
	}
}

func tableExists(t *testing.T, database *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := database.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}

	return name == table
}
