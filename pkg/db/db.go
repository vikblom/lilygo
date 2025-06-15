package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	_, err = db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS image (
			id TEXT PRIMARY KEY,
			png BLOB NOT NULL,
			t TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &DB{db}, nil
}

func (db *DB) AddImage(ctx context.Context, bs []byte) (uuid.UUID, error) {
	id := uuid.New()
	_, err := db.db.ExecContext(ctx,
		`INSERT INTO image (id, png) VALUES (?,?);`, id, bs,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert: %w", err)
	}
	return id, nil
}

func (db *DB) RandomImage(ctx context.Context) (uuid.UUID, error) {
	row := db.db.QueryRowContext(ctx,
		`SELECT id FROM image ORDER BY random() LIMIT 1 ;`,
	)

	var v uuid.UUID
	if err := row.Scan(&v); err != nil {
		return uuid.Nil, fmt.Errorf("query: %w", err)
	}

	return v, nil
}

func (db *DB) ReadImage(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row := db.db.QueryRowContext(ctx,
		`SELECT png FROM image WHERE id = ? ;`, id,
	)

	var v []byte
	if err := row.Scan(&v); err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	return v, nil
}

func (db *DB) ListImages(ctx context.Context) ([]uuid.UUID, error) {
	row, err := db.db.QueryContext(ctx,
		`SELECT id FROM image ORDER BY t desc;`,
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var out []uuid.UUID
	for row.Next() {
		err := row.Err()
		if err != nil {
			return nil, fmt.Errorf("row: %w", err)
		}
		var v uuid.UUID
		err = row.Scan(&v)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, v)
	}

	return out, nil
}
