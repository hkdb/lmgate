package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	if err := runColumnMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running column migrations: %w", err)
	}

	return db, nil
}

func hasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func runColumnMigrations(db *sql.DB) error {
	if !hasColumn(db, "oidc_providers", "name") {
		if _, err := db.Exec(`ALTER TABLE oidc_providers ADD COLUMN name TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("adding name column to oidc_providers: %w", err)
		}
		if _, err := db.Exec(`UPDATE oidc_providers SET name = provider_type WHERE name = ''`); err != nil {
			return fmt.Errorf("backfilling name column: %w", err)
		}
	}

	if !hasColumn(db, "model_acls", "group_id") {
		if _, err := db.Exec(`ALTER TABLE model_acls ADD COLUMN group_id TEXT REFERENCES groups(id) ON DELETE CASCADE`); err != nil {
			return fmt.Errorf("adding group_id column to model_acls: %w", err)
		}
		if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_model_acls_group ON model_acls(group_id)`); err != nil {
			return fmt.Errorf("creating index on model_acls.group_id: %w", err)
		}
	}

	if !hasColumn(db, "oidc_providers", "groups_claim") {
		if _, err := db.Exec(`ALTER TABLE oidc_providers ADD COLUMN groups_claim TEXT NOT NULL DEFAULT 'groups'`); err != nil {
			return fmt.Errorf("adding groups_claim column to oidc_providers: %w", err)
		}
	}

	if !hasColumn(db, "oidc_providers", "required_group") {
		if _, err := db.Exec(`ALTER TABLE oidc_providers ADD COLUMN required_group TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("adding required_group column to oidc_providers: %w", err)
		}
	}

	if !hasColumn(db, "groups", "admin_role") {
		if _, err := db.Exec(`ALTER TABLE groups ADD COLUMN admin_role INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("adding admin_role column to groups: %w", err)
		}
	}

	return nil
}

func runMigrations(db *sql.DB) error {
	stmts := strings.Split(schema, ";")
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("executing migration statement: %s: %w", stmt[:min(len(stmt), 60)], err)
		}
	}

	return nil
}
