package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"orenkod/backend/migrations"
)

func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	waitCtx, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	for {
		if err = db.PingContext(waitCtx); err == nil {
			break
		}
		if waitCtx.Err() != nil {
			_ = db.Close()
			return nil, fmt.Errorf("can't connect to PostgreSQL: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	if err = ApplySchema(waitCtx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = Seed(waitCtx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = SeedExtras(waitCtx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ApplySchema(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[len(entry.Name())-4:] == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		schema, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(schema)); err != nil {
			return fmt.Errorf("apply schema %s: %w", name, err)
		}
	}
	return nil
}
